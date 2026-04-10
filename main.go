package main

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexflint/go-arg"
	"github.com/sydlexius/mxlrcsvc-go/internal/app"
	"github.com/sydlexius/mxlrcsvc-go/internal/lyrics"
	"github.com/sydlexius/mxlrcsvc-go/internal/musixmatch"
	"github.com/sydlexius/mxlrcsvc-go/internal/scanner"
)

// Args defines the CLI arguments for the application.
type Args struct {
	Song     []string `arg:"positional,required" help:"song information in [ artist,title ] format (required)"`
	Outdir   string   `arg:"-o,--outdir" help:"output directory" default:"lyrics"`
	Cooldown int      `arg:"-c,--cooldown" help:"cooldown time in seconds" default:"15"`
	Depth    int      `arg:"-d,--depth" help:"(directory mode) maximum recursion depth" default:"100"`
	Update   bool     `arg:"-u,--update" help:"(directory mode) update existing lyrics file"`
	BFS      bool     `arg:"--bfs" help:"(directory mode) use breadth-first-search traversal"`
	Token    string   `arg:"-t,--token" help:"musixmatch token" default:""`
}

var inputs = app.NewInputsQueue()
var failed = app.NewInputsQueue()

func main() {
	var args Args
	arg.MustParse(&args)

	sc := scanner.NewScanner()
	mode, err := sc.ParseInput(args.Song, args.Outdir, args.Update, args.Depth, args.BFS, inputs)
	if err != nil {
		slog.Error("failed to parse input", "error", err)
		os.Exit(1)
	}
	cnt := inputs.Len()
	fmt.Printf("\n%d lyrics to fetch\n\n", cnt)

	if mode == "dir" {
		args.Outdir = ""
	} else {
		if err := os.MkdirAll(args.Outdir, 0750); err != nil { //nolint:gosec // user-specified output directory
			slog.Error("failed to create output directory", "error", err)
			os.Exit(1)
		}
	}

	closeHandler(mode, cnt)
	var token string
	if token = args.Token; args.Token == "" {
		token = "2203269256ff7abcb649269df00e14c833dbf4ddfb5b36a1aae8b0"
	}
	mx := musixmatch.NewClient(token)
	w := lyrics.NewLRCWriter()

	for !inputs.Empty() {
		cur := inputs.Next()
		slog.Info("searching song", "artist", cur.Track.ArtistName, "track", cur.Track.TrackName)
		song, err := mx.FindLyrics(cur.Track)
		if err == nil {
			slog.Info("formatting lyrics")
			writeErr := w.WriteLRC(song, cur.Filename, cur.Outdir)
			cur = inputs.Pop()
			if writeErr != nil {
				slog.Error("failed to save lyrics", "error", writeErr)
				failed.Push(cur)
			}
		} else {
			slog.Error("lyrics fetch failed", "error", err)
			failed.Push(inputs.Pop())
		}
		timer(args.Cooldown, inputs.Len())
	}
	if !failed.Empty() {
		failedHandler(mode, cnt)
	}
}

func timer(maxSec int, n int) {
	if n <= 0 {
		return
	}
	for i := maxSec; i >= 0; i-- {
		fmt.Printf("    Please wait... %ds    \r", i)
		time.Sleep(time.Second)
	}
	fmt.Printf("\n\n")
}

func failedHandler(mode string, cnt int) {
	fmt.Printf("\n")
	if !inputs.Empty() {
		failed.Queue = append(failed.Queue, inputs.Queue...)
	}
	slog.Info("fetch complete", "success", cnt-failed.Len(), "total", cnt)
	if failed.Empty() {
		return
	}
	slog.Info("failed to fetch lyrics", "count", failed.Len())

	if mode == "dir" {
		slog.Info("you can try again with the same command")
	} else {
		t := time.Now().Format("20060102_150405")
		fn := t + "_failed.txt"
		slog.Info("saving list of failed items", "file", fn)

		f, err := os.Create(fn) //nolint:gosec // filename is generated from timestamp, not user input
		if err != nil {
			slog.Error("failed to create failed items file", "error", err)
			os.Exit(1)
		}

		buffer := bufio.NewWriter(f)
		for !failed.Empty() {
			cur := failed.Pop()
			_, err := buffer.WriteString(cur.Track.ArtistName + "," + cur.Track.TrackName + "\n")
			if err != nil {
				slog.Error("failed to write failed item", "error", err)
				os.Exit(1)
			}
		}
		if err := buffer.Flush(); err != nil {
			slog.Error("failed to flush failed items", "error", err)
			os.Exit(1)
		}
		if err := f.Close(); err != nil {
			slog.Error("error saving failed items file", "error", err)
			os.Exit(1)
		}
	}
}

func closeHandler(mode string, cnt int) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		fmt.Printf("\n")
		failedHandler(mode, cnt)
		os.Exit(0)
	}()
}
