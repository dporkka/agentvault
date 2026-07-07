package indexer

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/embeddings"
)

func setupTestVault(t *testing.T) (string, *db.DB, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		t.Fatalf("failed to create .agentvault dir: %v", err)
	}
	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("failed to open db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}
	cleanup := func() {
		database.Close()
	}
	return tmpDir, database, cleanup
}

func writeNote(t *testing.T, vaultPath, relPath, content string) {
	t.Helper()
	fullPath := filepath.Join(vaultPath, relPath)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write note %s: %v", relPath, err)
	}
}

func mustIndex(t *testing.T, idx *Indexer, opts IndexOptions) *IndexResult {
	t.Helper()
	result, err := idx.Index(opts)
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	return result
}

func TestIndexNewFile(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nThis is a great idea.")

	result := mustIndex(t, idx, IndexOptions{})
	if result.Scanned != 1 {
		t.Errorf("expected Scanned=1, got %d", result.Scanned)
	}
	if result.Added != 1 {
		t.Errorf("expected Added=1, got %d", result.Added)
	}
	if result.Skipped != 0 {
		t.Errorf("expected Skipped=0, got %d", result.Skipped)
	}
	if result.Updated != 0 {
		t.Errorf("expected Updated=0, got %d", result.Updated)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestIndexSkipsUnchangedFile(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nThis is a great idea.")

	mustIndex(t, idx, IndexOptions{})
	result := mustIndex(t, idx, IndexOptions{})

	if result.Added != 0 {
		t.Errorf("expected Added=0, got %d", result.Added)
	}
	if result.Skipped != 1 {
		t.Errorf("expected Skipped=1, got %d", result.Skipped)
	}
	if result.Updated != 0 {
		t.Errorf("expected Updated=0, got %d", result.Updated)
	}
}

func TestIndexForceReindexes(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nThis is a great idea.")

	mustIndex(t, idx, IndexOptions{})
	result := mustIndex(t, idx, IndexOptions{Force: true})

	if result.Skipped != 0 {
		t.Errorf("expected Skipped=0 with Force=true, got %d", result.Skipped)
	}
	if result.Updated != 1 {
		t.Errorf("expected Updated=1 with Force=true, got %d", result.Updated)
	}
}

func TestIndexUpdatesChangedFile(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nOriginal content.")
	mustIndex(t, idx, IndexOptions{})

	// Small delay to ensure updated_at changes
	time.Sleep(10 * time.Millisecond)
	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nUpdated content.")

	result := mustIndex(t, idx, IndexOptions{})
	if result.Updated != 1 {
		t.Errorf("expected Updated=1, got %d", result.Updated)
	}
	if result.Skipped != 0 {
		t.Errorf("expected Skipped=0, got %d", result.Skipped)
	}
}

func TestIndexPathFiltering(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: Note\ntype: note\n---\n\nContent.")
	writeNote(t, vaultPath, "20-projects/project.md", "---\ntitle: Project\ntype: project\n---\n\nContent.")

	result := mustIndex(t, idx, IndexOptions{Path: "20-projects"})
	if result.Scanned != 1 {
		t.Errorf("expected Scanned=1 for subpath, got %d", result.Scanned)
	}
	if result.Added != 1 {
		t.Errorf("expected Added=1, got %d", result.Added)
	}
}

func TestIndexRebuild(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nContent.")
	mustIndex(t, idx, IndexOptions{})

	result := mustIndex(t, idx, IndexOptions{Rebuild: true})
	if result.Updated != 1 {
		t.Errorf("expected Updated=1 for rebuild, got %d", result.Updated)
	}
	if result.Skipped != 0 {
		t.Errorf("expected Skipped=0 for rebuild, got %d", result.Skipped)
	}
}

func TestIndexHiddenDirectorySkipped(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, ".hidden/note.md", "---\ntitle: Hidden\ntype: note\n---\n\nContent.")
	writeNote(t, vaultPath, "10-notes/visible.md", "---\ntitle: Visible\ntype: note\n---\n\nContent.")

	result := mustIndex(t, idx, IndexOptions{})
	if result.Scanned != 1 {
		t.Errorf("expected Scanned=1 (hidden dir skipped), got %d", result.Scanned)
	}
}

func TestIndexInvalidFrontmatter(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/bad.md", "---\nthis is not: valid: yaml: :\n---\n\nContent.")

	result, err := idx.Index(IndexOptions{})
	if err != nil {
		t.Fatalf("Index returned unexpected error: %v", err)
	}
	if result.Scanned != 1 {
		t.Errorf("expected Scanned=1, got %d", result.Scanned)
	}
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error, got %d: %v", len(result.Errors), result.Errors)
	}
}

func TestIndexCleanupDeletedFiles(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nContent.")
	mustIndex(t, idx, IndexOptions{})

	if err := os.Remove(filepath.Join(vaultPath, "10-notes/idea.md")); err != nil {
		t.Fatalf("failed to remove note: %v", err)
	}

	result := mustIndex(t, idx, IndexOptions{})
	if result.Scanned != 0 {
		t.Errorf("expected Scanned=0 after deletion, got %d", result.Scanned)
	}

	var count int
	row := database.QueryRow("SELECT COUNT(*) FROM files")
	if err := row.Scan(&count); err != nil {
		t.Fatalf("failed to count files: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 files in db, got %d", count)
	}
}

func TestIndexWithEmbeddings(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nContent long enough to chunk.")

	client := embeddings.NewClient("http://localhost:11434", "nomic-embed-text")
	client.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := map[string]interface{}{
				"embedding": []float32{0.1, 0.2, 0.3},
			}
			b, _ := json.Marshal(body)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(b)),
				Header:     make(http.Header),
			}, nil
		}),
	})

	embedCfg := &EmbedConfig{Enabled: true, Client: client}
	result, err := idx.Index(IndexOptions{Embed: true, embedCfg: embedCfg})
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	if result.Added != 1 {
		t.Errorf("expected Added=1, got %d", result.Added)
	}
	if result.EmbedErrors != 0 {
		t.Errorf("expected 0 embed errors, got %d", result.EmbedErrors)
	}
	if result.ChunksAdded == 0 {
		t.Error("expected chunks to be added")
	}

	var chunkCount int
	row := database.QueryRow("SELECT COUNT(*) FROM chunks")
	if err := row.Scan(&chunkCount); err != nil {
		t.Fatalf("failed to count chunks: %v", err)
	}
	if chunkCount == 0 {
		t.Error("expected chunks in database")
	}
}

func TestIndexWithEmbeddingError(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nContent long enough to chunk.")

	client := embeddings.NewClient("http://localhost:11434", "nomic-embed-text")
	client.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: 500,
				Body:       io.NopCloser(bytes.NewReader([]byte("error"))),
				Header:     make(http.Header),
			}, nil
		}),
	})

	embedCfg := &EmbedConfig{Enabled: true, Client: client}
	result, err := idx.Index(IndexOptions{Embed: true, embedCfg: embedCfg})
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	if result.Added != 1 {
		t.Errorf("expected Added=1, got %d", result.Added)
	}
	if result.EmbedErrors != 1 {
		t.Errorf("expected 1 embed error, got %d", result.EmbedErrors)
	}
}

func TestComputeHash(t *testing.T) {
	h1 := ComputeHash([]byte("hello"))
	h2 := ComputeHash([]byte("hello"))
	h3 := ComputeHash([]byte("world"))
	if h1 != h2 {
		t.Error("same content should produce same hash")
	}
	if h1 == h3 {
		t.Error("different content should produce different hashes")
	}
	if len(h1) != 64 {
		t.Errorf("expected sha256 hex length 64, got %d", len(h1))
	}
}

func TestFilepathToID(t *testing.T) {
	id := filepathToID("10-notes/my idea.md")
	if id != "10-notes_my idea" {
		t.Errorf("expected '10-notes_my idea', got %q", id)
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
