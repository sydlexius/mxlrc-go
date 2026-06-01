package watcher

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rjeczalik/notify"

	"github.com/sydlexius/mxlrcgo-svc/internal/models"
)

// LibraryLister lists configured library roots.
type LibraryLister interface {
	List(ctx context.Context) ([]models.Library, error)
}

// ScanFunc performs a targeted scan of path on behalf of lib.
type ScanFunc func(ctx context.Context, lib models.Library, path string) error

// Watcher watches configured library roots and triggers targeted scans.
type Watcher struct {
	cfg       Config
	libraries LibraryLister
	scan      ScanFunc
}

// New creates a Watcher. scan is invoked (after debouncing) with the owning
// library and the directory that changed.
func New(cfg Config, libraries LibraryLister, scan ScanFunc) *Watcher {
	return &Watcher{cfg: cfg, libraries: libraries, scan: scan}
}

// libEvent is a debounced, library-resolved scan request.
type libEvent struct {
	lib  models.Library
	path string
}

// Run registers recursive watches for every configured library root and
// dispatches debounced scans until ctx is canceled. It fails fast if the watch
// budget would be exceeded rather than silently truncating coverage.
func (w *Watcher) Run(ctx context.Context) error {
	libs, err := w.libraries.List(ctx)
	if err != nil {
		return fmt.Errorf("watcher: list libraries: %w", err)
	}
	if len(libs) == 0 {
		slog.Info("watcher: no libraries configured; nothing to watch")
		return nil
	}

	dirs, err := countDirs(libs)
	if err != nil {
		return err
	}
	if dirs > w.cfg.MaxDirs {
		return fmt.Errorf("watcher: %d directories under configured roots exceed %s=%d; raise the limit or narrow the roots", dirs, EnvMaxDirs, w.cfg.MaxDirs)
	}

	c := make(chan notify.EventInfo, eventBuffer)
	for _, lib := range libs {
		// "<root>/..." asks notify for a recursive watch over the subtree,
		// which also covers directories created after registration.
		if err := notify.Watch(filepath.Join(lib.Path, "..."), c, notify.Create, notify.Write, notify.Rename, notify.Remove); err != nil {
			notify.Stop(c)
			return fmt.Errorf("watcher: watch %s: %w", lib.Path, err)
		}
	}
	defer notify.Stop(c)
	slog.Info("watcher started", "libraries", len(libs), "directories", dirs, "debounce", w.cfg.Debounce)

	events := make(chan libEvent)
	go w.translate(ctx, c, libs, events)
	w.dispatch(ctx, events)
	return ctx.Err()
}

const eventBuffer = 1024

// translate maps raw filesystem events to library-resolved scan targets and
// forwards them until ctx is canceled.
func (w *Watcher) translate(ctx context.Context, c <-chan notify.EventInfo, libs []models.Library, out chan<- libEvent) {
	for {
		select {
		case <-ctx.Done():
			return
		case ei := <-c:
			lib, dir, ok := eventTarget(libs, ei.Path())
			if !ok {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- libEvent{lib: lib, path: dir}:
			}
		}
	}
}

// dispatch debounces incoming events and runs a scan per touched directory once
// the quiet period elapses. A sliding window coalesces bursts: each new event
// resets the timer, so a tagger rewriting an album triggers a single scan.
func (w *Watcher) dispatch(ctx context.Context, events <-chan libEvent) {
	pending := make(map[string]models.Library)
	var timer *time.Timer
	var timerC <-chan time.Time

	flush := func() {
		for path, lib := range pending {
			if err := w.scan(ctx, lib, path); err != nil {
				slog.Warn("watcher scan failed", "path", path, "library", lib.ID, "error", err)
			}
			delete(pending, path)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-events:
			pending[ev.path] = ev.lib
			if timer != nil {
				timer.Stop()
			}
			timer = time.NewTimer(w.cfg.Debounce)
			timerC = timer.C
		case <-timerC:
			flush()
			timer = nil
			timerC = nil
		}
	}
}

// eventTarget returns the library that owns path and the directory to scan. A
// file event scans the file's directory; a directory event scans that
// directory. When path no longer exists (delete/rename), its parent directory
// is scanned so the removal is reconciled. ok is false when no configured
// library contains path.
func eventTarget(libs []models.Library, path string) (models.Library, string, bool) {
	var best models.Library
	found := false
	for _, lib := range libs {
		if pathWithin(lib.Path, path) && (!found || len(lib.Path) > len(best.Path)) {
			best = lib
			found = true
		}
	}
	if !found {
		return models.Library{}, "", false
	}
	dir := path
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		dir = filepath.Dir(path)
	}
	return best, dir, true
}

// pathWithin reports whether p is root or sits under root.
func pathWithin(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	if err != nil {
		return false
	}
	return rel == "." || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && rel != ".."
}

// countDirs returns the number of directories under the library roots, used to
// enforce the watch budget before any watches are registered.
func countDirs(libs []models.Library) (int, error) {
	total := 0
	for _, lib := range libs {
		err := filepath.WalkDir(lib.Path, func(_ string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				total++
			}
			return nil
		})
		if err != nil {
			return 0, fmt.Errorf("watcher: count directories under %s: %w", lib.Path, err)
		}
	}
	return total, nil
}
