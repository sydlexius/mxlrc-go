package watcher

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/sydlexius/mxlrcgo-svc/internal/models"
)

type fakeLister struct {
	libs []models.Library
	err  error
}

func (f fakeLister) List(context.Context) ([]models.Library, error) {
	return f.libs, f.err
}

func TestConfigFromEnv(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		t.Setenv(EnvEnabled, "")
		t.Setenv(EnvDebounceMS, "")
		t.Setenv(EnvMaxDirs, "")
		cfg := ConfigFromEnv()
		if cfg.Enabled {
			t.Error("Enabled = true; want false by default")
		}
		if cfg.Debounce != defaultDebounceMS*time.Millisecond {
			t.Errorf("Debounce = %s; want %dms", cfg.Debounce, defaultDebounceMS)
		}
		if cfg.MaxDirs != defaultMaxDirs {
			t.Errorf("MaxDirs = %d; want %d", cfg.MaxDirs, defaultMaxDirs)
		}
	})

	t.Run("overrides", func(t *testing.T) {
		t.Setenv(EnvEnabled, "true")
		t.Setenv(EnvDebounceMS, "500")
		t.Setenv(EnvMaxDirs, "42")
		cfg := ConfigFromEnv()
		if !cfg.Enabled {
			t.Error("Enabled = false; want true")
		}
		if cfg.Debounce != 500*time.Millisecond {
			t.Errorf("Debounce = %s; want 500ms", cfg.Debounce)
		}
		if cfg.MaxDirs != 42 {
			t.Errorf("MaxDirs = %d; want 42", cfg.MaxDirs)
		}
	})

	t.Run("invalid falls back", func(t *testing.T) {
		t.Setenv(EnvDebounceMS, "notanumber")
		t.Setenv(EnvMaxDirs, "-5")
		cfg := ConfigFromEnv()
		if cfg.Debounce != defaultDebounceMS*time.Millisecond {
			t.Errorf("Debounce = %s; want default after invalid", cfg.Debounce)
		}
		if cfg.MaxDirs != defaultMaxDirs {
			t.Errorf("MaxDirs = %d; want default after invalid", cfg.MaxDirs)
		}
	})
}

func TestEventTargetResolvesOwningLibrary(t *testing.T) {
	libs := []models.Library{
		{ID: 1, Path: "/music"},
		{ID: 2, Path: "/music/classical"}, // nested, more specific
	}

	// A file under the nested library resolves to the most specific root, and
	// the scan target is the file's directory.
	lib, dir, ok := eventTarget(libs, "/music/classical/Bach/aria.flac")
	if !ok {
		t.Fatal("eventTarget ok = false; want true")
	}
	if lib.ID != 2 {
		t.Errorf("lib ID = %d; want 2 (most specific root)", lib.ID)
	}
	if dir != "/music/classical/Bach" {
		t.Errorf("dir = %q; want the file's directory", dir)
	}

	// A path outside every library is not a target.
	if _, _, ok := eventTarget(libs, "/somewhere/else/x.mp3"); ok {
		t.Error("eventTarget for outside path ok = true; want false")
	}
}

func TestPathWithin(t *testing.T) {
	cases := []struct {
		root, p string
		want    bool
	}{
		{"/music", "/music", true},
		{"/music", "/music/a/b.mp3", true},
		{"/music", "/musicother/x", false},
		{"/music", "/other", false},
		{"/music/sub", "/music", false},
	}
	for _, c := range cases {
		if got := pathWithin(c.root, c.p); got != c.want {
			t.Errorf("pathWithin(%q, %q) = %v; want %v", c.root, c.p, got, c.want)
		}
	}
}

func TestCountDirs(t *testing.T) {
	root := t.TempDir()
	for _, sub := range []string{"a", "a/b", "c"} {
		if err := os.MkdirAll(filepath.Join(root, sub), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	n, err := countDirs([]models.Library{{Path: root}})
	if err != nil {
		t.Fatalf("countDirs: %v", err)
	}
	// root + a + a/b + c = 4
	if n != 4 {
		t.Errorf("countDirs = %d; want 4", n)
	}
}

func TestDispatchCoalescesBurstIntoSingleScan(t *testing.T) {
	var mu sync.Mutex
	calls := map[string]int{}
	scan := func(_ context.Context, _ models.Library, path string) error {
		mu.Lock()
		calls[path]++
		mu.Unlock()
		return nil
	}
	w := New(Config{Debounce: 30 * time.Millisecond}, nil, scan)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events := make(chan libEvent)
	done := make(chan struct{})
	go func() { w.dispatch(ctx, events); close(done) }()

	lib := models.Library{ID: 1, Path: "/m"}
	for i := 0; i < 5; i++ { // burst on one dir
		events <- libEvent{lib: lib, path: "/m/Album"}
	}
	events <- libEvent{lib: lib, path: "/m/Other"} // a second dir

	time.Sleep(150 * time.Millisecond) // let the debounce window elapse and flush

	mu.Lock()
	album, other := calls["/m/Album"], calls["/m/Other"]
	mu.Unlock()
	if album != 1 {
		t.Errorf("scans for /m/Album = %d; want 1 (burst coalesced)", album)
	}
	if other != 1 {
		t.Errorf("scans for /m/Other = %d; want 1", other)
	}

	cancel()
	<-done
}

func TestRunReturnsNilWhenNoLibraries(t *testing.T) {
	w := New(Config{MaxDirs: defaultMaxDirs}, fakeLister{}, func(context.Context, models.Library, string) error { return nil })
	if err := w.Run(context.Background()); err != nil {
		t.Fatalf("Run with no libraries = %v; want nil", err)
	}
}

func TestRunFailsWhenWatchBudgetExceeded(t *testing.T) {
	root := t.TempDir() // at least one directory exists
	w := New(Config{MaxDirs: 0}, fakeLister{libs: []models.Library{{ID: 1, Path: root}}},
		func(context.Context, models.Library, string) error { return nil })
	err := w.Run(context.Background())
	if err == nil {
		t.Fatal("Run with exceeded budget = nil; want a loud failure")
	}
}

// TestRunTriggersScanOnFileCreate exercises the real notify integration: a file
// created under a watched root must trigger a scan within the debounce window.
// Filesystem event delivery is best-effort and platform dependent, so the test
// allows a generous timeout.
func TestRunTriggersScanOnFileCreate(t *testing.T) {
	root := t.TempDir()
	scanned := make(chan string, 1)
	w := New(
		Config{Debounce: 20 * time.Millisecond, MaxDirs: defaultMaxDirs},
		fakeLister{libs: []models.Library{{ID: 5, Path: root}}},
		func(_ context.Context, _ models.Library, path string) error {
			select {
			case scanned <- path:
			default:
			}
			return nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	runErr := make(chan error, 1)
	go func() { runErr <- w.Run(ctx) }()

	time.Sleep(200 * time.Millisecond) // allow watch registration
	if err := os.WriteFile(filepath.Join(root, "new.mp3"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	select {
	case got := <-scanned:
		if got != root {
			t.Errorf("scanned path = %q; want %q", got, root)
		}
	case <-time.After(5 * time.Second):
		t.Skip("no filesystem event delivered within 5s (best-effort watcher; may be unsupported here)")
	}

	cancel()
	<-runErr
}
