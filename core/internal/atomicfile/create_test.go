package atomicfile

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// The final capture path must remain absent until the complete staged file is
// publishable. The old O_EXCL zero-byte reservation fails this requirement.
func TestWriteNewFilePublishesOnlyCompleteContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture_001.md")
	body := bytes.Repeat([]byte("complete capture\n"), 128)
	entered := false
	err := writeNewFile(path, body, 0644, func(stage, destination string) error {
		entered = true
		if _, statErr := os.Lstat(destination); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("destination exists before publication: %v", statErr)
		}
		staged, readErr := os.ReadFile(stage)
		if readErr != nil || !bytes.Equal(staged, body) {
			t.Fatalf("staged content incomplete: %v", readErr)
		}
		return os.Link(stage, destination)
	})
	if err != nil {
		t.Fatal(err)
	}
	if !entered {
		t.Fatal("publication hook did not execute")
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, body) {
		t.Fatalf("published file differs from staged contents: %v", err)
	}
	assertNoStagingFiles(t, dir)
}

func TestWriteNewFileExistingDestinationIsNeverReplaced(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture_001.md")
	if err := os.WriteFile(path, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := WriteNewFile(path, []byte("new"), 0644); !errors.Is(err, fs.ErrExist) {
		t.Fatalf("expected collision, got %v", err)
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != "original" {
		t.Fatalf("existing capture modified: %q, %v", actual, err)
	}
	assertNoStagingFiles(t, dir)
}

func TestWriteNewFileConcurrentWritersHaveOneWinner(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture_001.md")
	const writers = 12
	var wg sync.WaitGroup
	results := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- WriteNewFile(path, []byte("one whole capture\n"), 0644)
		}()
	}
	wg.Wait()
	close(results)
	successes, collisions := 0, 0
	for err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, fs.ErrExist):
			collisions++
		default:
			t.Fatalf("unexpected installation error: %v", err)
		}
	}
	if successes != 1 || collisions != writers-1 {
		t.Fatalf("wanted one winner and %d collisions, got %d and %d", writers-1, successes, collisions)
	}
	actual, err := os.ReadFile(path)
	if err != nil || string(actual) != "one whole capture\n" {
		t.Fatalf("incomplete published capture: %q, %v", actual, err)
	}
	assertNoStagingFiles(t, dir)
}

func TestWriteNewFileInstallFailureLeavesNoEmptyCapture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "capture_001.md")
	injected := errors.New("injected publication failure")
	err := writeNewFile(path, []byte("complete"), 0644, func(src, dst string) error {
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, statErr := os.Lstat(path); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("empty capture should not be published: %v", statErr)
	}
	assertNoStagingFiles(t, dir)
}

func TestWriteNewFileRejectsMissingDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "capture.md")
	if err := WriteNewFile(path, []byte("complete"), 0644); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("missing directory should fail: %v", err)
	}
}
