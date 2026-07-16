// Package watcher watches a vault directory for .md file changes and triggers
// async reindexing so search results stay current without manual indexing.
package watcher

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/agentvault/core/internal/indexer"
	"github.com/fsnotify/fsnotify"
)

// Watcher watches a vault directory tree for .md file changes and triggers
// async reindexing via the provided indexer.
type Watcher struct {
	w         *fsnotify.Watcher
	idx       *indexer.Indexer
	vaultPath string
	debounce  map[string]*time.Timer
	done      chan struct{}
	mu        sync.Mutex
}

// New creates a Watcher that monitors vaultPath recursively for .md file
// changes. It returns an error if setting up the filesystem watch fails.
func New(vaultPath string, idx *indexer.Indexer) (*Watcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	watcher := &Watcher{
		w:         w,
		idx:       idx,
		vaultPath: vaultPath,
		debounce:  make(map[string]*time.Timer),
		done:      make(chan struct{}),
	}

	if err := watcher.addRecursiveWatch(vaultPath); err != nil {
		w.Close()
		return nil, err
	}

	return watcher, nil
}

// Start begins processing filesystem events in a background goroutine.
// Call Stop to shut down cleanly.
func (w *Watcher) Start() {
	go w.loop()
}

// Stop shuts down the watcher and releases resources. It is safe to call
// multiple times.
func (w *Watcher) Stop() {
	select {
	case <-w.done:
		return // already stopped
	default:
		close(w.done)
	}
	w.w.Close()

	w.mu.Lock()
	for _, t := range w.debounce {
		t.Stop()
	}
	w.debounce = nil
	w.mu.Unlock()
}

// Watching returns true if the watcher is currently active.
func (w *Watcher) Watching() bool {
	select {
	case <-w.done:
		return false
	default:
		return true
	}
}

// loop is the main event loop. It runs in a background goroutine.
func (w *Watcher) loop() {
	const debounceInterval = 500 * time.Millisecond

	for {
		select {
		case event, ok := <-w.w.Events:
			if !ok {
				return
			}
			w.handleEvent(event, debounceInterval)

		case err, ok := <-w.w.Errors:
			if !ok {
				return
			}
			_ = err // silently ignore fsnotify errors

		case <-w.done:
			return
		}
	}
}

// handleEvent processes a single filesystem event with debouncing.
func (w *Watcher) handleEvent(event fsnotify.Event, interval time.Duration) {
	if !strings.HasSuffix(event.Name, ".md") {
		return
	}

	relPath, err := filepath.Rel(w.vaultPath, event.Name)
	if err != nil || relPath == "" {
		return
	}

	w.mu.Lock()
	if t, ok := w.debounce[relPath]; ok {
		t.Stop()
	}

	// For REMOVE events, use a full scan so cleanupDeletedFiles runs.
	// For CREATE/WRITE, index only the changed file.
	opts := indexer.IndexOptions{Path: relPath}
	if event.Has(fsnotify.Remove) {
		opts = indexer.IndexOptions{}
	}

	// Capture opts by value so each debounced callback uses the correct
	// options even if handleEvent is called again before the timer fires.
	idxOpts := opts
	w.debounce[relPath] = time.AfterFunc(interval, func() {
		w.mu.Lock()
		delete(w.debounce, relPath)
		w.mu.Unlock()
		w.idx.Index(idxOpts)
	})
	w.mu.Unlock()
}

// addRecursiveWatch adds a watch on root and all its subdirectories,
// skipping hidden directories.
func (w *Watcher) addRecursiveWatch(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		// Skip hidden directories (but not the root itself).
		if path != root && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}
		return w.w.Add(path)
	})
}
