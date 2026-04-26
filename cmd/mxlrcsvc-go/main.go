package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	arg "github.com/alexflint/go-arg"
	"github.com/joho/godotenv"
	"github.com/sydlexius/mxlrcsvc-go/internal/app"
	"github.com/sydlexius/mxlrcsvc-go/internal/config"
	"github.com/sydlexius/mxlrcsvc-go/internal/db"
	"github.com/sydlexius/mxlrcsvc-go/internal/lyrics"
	"github.com/sydlexius/mxlrcsvc-go/internal/musixmatch"
	"github.com/sydlexius/mxlrcsvc-go/internal/queue"
	"github.com/sydlexius/mxlrcsvc-go/internal/scanner"
)

// Args defines the CLI arguments for the application.
type Args struct {
	Song       []string `arg:"positional,required" help:"song information in [ artist,title ] format (required)"`
	Outdir     *string  `arg:"-o,--outdir" help:"output directory (default: from config or 'lyrics')"`
	Cooldown   *int     `arg:"-c,--cooldown" help:"cooldown time in seconds (default: from config or 15)"`
	Depth      int      `arg:"-d,--depth" help:"(directory mode) maximum recursion depth" default:"100"`
	Update     bool     `arg:"-u,--update" help:"(directory mode) re-fetch and overwrite existing .lrc files"`
	Upgrade    bool     `arg:"--upgrade" help:"(directory mode) re-fetch songs with .txt (unsynced) to promote to .lrc if synced lyrics are now available; implied by --update"`
	BFS        bool     `arg:"--bfs" help:"(directory mode) use breadth-first-search traversal"`
	Token      string   `arg:"-t,--token" help:"musixmatch token (or MUSIXMATCH_TOKEN / MXLRC_API_TOKEN env var, or config file)" default:""`
	ConfigPath string   `arg:"--config" help:"path to config file (default: XDG)" default:""`
}

func main() {
	os.Exit(run())
}

type appRunner interface {
	Run(ctx context.Context) error
}

type runOptions struct {
	args       []string
	out        io.Writer
	ctx        context.Context
	loadDotenv func() error
	newFetcher func(token string) musixmatch.Fetcher
	newWriter  func() lyrics.Writer
	newApp     func(fetcher musixmatch.Fetcher, writer lyrics.Writer, inputs *queue.InputsQueue, cooldown int, mode string) appRunner
}

// run executes the application and returns an exit code.
// Using a helper function ensures deferred cleanup (e.g. sqlDB.Close) runs
// before os.Exit is called, while still producing a non-zero exit on error.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	return runWithOptions(runOptions{ctx: ctx})
}

func runWithOptions(opts runOptions) int {
	rawArgs := opts.args
	if rawArgs == nil {
		rawArgs = os.Args[1:]
	}
	out := opts.out
	if out == nil {
		out = os.Stdout
	}
	ctx := opts.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	loadDotenv := opts.loadDotenv
	if loadDotenv == nil {
		loadDotenv = func() error { return godotenv.Load() }
	}
	newFetcher := opts.newFetcher
	if newFetcher == nil {
		newFetcher = func(token string) musixmatch.Fetcher { return musixmatch.NewClient(token) }
	}
	newWriter := opts.newWriter
	if newWriter == nil {
		newWriter = func() lyrics.Writer { return lyrics.NewLRCWriter() }
	}
	newApp := opts.newApp
	if newApp == nil {
		newApp = func(fetcher musixmatch.Fetcher, writer lyrics.Writer, inputs *queue.InputsQueue, cooldown int, mode string) appRunner {
			return app.NewApp(fetcher, writer, inputs, cooldown, mode)
		}
	}

	var args Args
	parser, err := arg.NewParser(arg.Config{Program: "mxlrcsvc-go", Out: out}, &args)
	if err != nil {
		_, _ = fmt.Fprintln(out, err)
		return 2
	}
	if err := parser.Parse(rawArgs); err != nil {
		if err == arg.ErrHelp {
			if err := parser.WriteHelpForSubcommand(out); err != nil {
				_, _ = fmt.Fprintln(out, err)
				return 2
			}
			return 0
		}
		if usageErr := parser.WriteUsageForSubcommand(out); usageErr != nil {
			_, _ = fmt.Fprintln(out, usageErr)
			return 2
		}
		_, _ = fmt.Fprintln(out, err)
		return 2
	}

	// Load .env file if present (does NOT overwrite existing env vars).
	// Error is ignored -- .env file is optional.
	_ = loadDotenv()

	cfg, err := config.Load(args.ConfigPath)
	if err != nil {
		slog.Error("failed to load config", "error", err)
		return 1
	}

	// Token precedence: CLI flag > env vars (handled in config.Load) > config file.
	token := args.Token
	if token == "" {
		token = cfg.API.Token
	}
	if token == "" {
		slog.Error("no API token provided: use --token flag, MUSIXMATCH_TOKEN env var, MXLRC_API_TOKEN env var, or config file")
		return 1
	}

	// Cooldown: explicit CLI flag wins; otherwise use config (which has its own default).
	cooldown := cfg.API.Cooldown
	if args.Cooldown != nil {
		cooldown = *args.Cooldown
	}

	// Outdir: explicit CLI flag wins; otherwise use config (which has its own default).
	outdir := cfg.Output.Dir
	if args.Outdir != nil {
		outdir = *args.Outdir
	}

	inputs := queue.NewInputsQueue()
	sc := scanner.NewScanner()
	mode, err := sc.ParseInput(args.Song, outdir, args.Update, args.Upgrade, args.Depth, args.BFS, inputs)
	if err != nil {
		slog.Error("failed to parse input", "error", err)
		return 1
	}
	fmt.Printf("\n%d lyrics to fetch\n\n", inputs.Len())

	if mode != "dir" {
		if err := os.MkdirAll(outdir, 0750); err != nil { //nolint:gosec // user-specified output directory
			slog.Error("failed to create output directory", "error", err)
			return 1
		}
	}

	sqlDB, err := db.Open(ctx, cfg.DB.Path)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		return 1
	}
	defer sqlDB.Close() //nolint:errcheck // best-effort close on shutdown

	mx := newFetcher(token)
	w := newWriter()
	application := newApp(mx, w, inputs, cooldown, mode)

	if err := application.Run(ctx); err != nil {
		slog.Error("application error", "error", err)
		return 1
	}
	return 0
}
