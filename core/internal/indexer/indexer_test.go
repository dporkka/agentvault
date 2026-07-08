package indexer

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/config"
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

// ============================================================================
// EmbedConfig tests
// ============================================================================

func TestBuildEmbedConfig(t *testing.T) {
	t.Run("no config uses defaults", func(t *testing.T) {
		vaultPath, database, cleanup := setupTestVault(t)
		defer cleanup()
		idx := New(database, vaultPath)

		cfg := idx.buildEmbedConfig()
		if cfg == nil {
			t.Fatal("expected non-nil EmbedConfig")
		}
		if !cfg.Enabled {
			t.Error("expected Enabled=true")
		}
		if cfg.Client.BaseURL() != "http://localhost:11434" {
			t.Errorf("BaseURL = %q, want %q", cfg.Client.BaseURL(), "http://localhost:11434")
		}
		if cfg.Client.Model() != "nomic-embed-text" {
			t.Errorf("Model = %q, want %q", cfg.Client.Model(), "nomic-embed-text")
		}
	})

	t.Run("config values are used", func(t *testing.T) {
		vaultPath, database, cleanup := setupTestVault(t)
		defer cleanup()
		idx := New(database, vaultPath)

		if err := config.Save(vaultPath, &config.VaultConfig{
			AI: &config.AIConfig{
				BaseURL:        "http://ollama:11434",
				EmbeddingModel: "all-minilm",
			},
		}); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		cfg := idx.buildEmbedConfig()
		if cfg == nil {
			t.Fatal("expected non-nil EmbedConfig")
		}
		if cfg.Client.BaseURL() != "http://ollama:11434" {
			t.Errorf("BaseURL = %q, want %q", cfg.Client.BaseURL(), "http://ollama:11434")
		}
		if cfg.Client.Model() != "all-minilm" {
			t.Errorf("Model = %q, want %q", cfg.Client.Model(), "all-minilm")
		}
	})

	t.Run("config without AI section uses defaults", func(t *testing.T) {
		vaultPath, database, cleanup := setupTestVault(t)
		defer cleanup()
		idx := New(database, vaultPath)

		if err := config.Save(vaultPath, &config.VaultConfig{}); err != nil {
			t.Fatalf("failed to save config: %v", err)
		}

		cfg := idx.buildEmbedConfig()
		if cfg.Client.Model() != "nomic-embed-text" {
			t.Errorf("Model = %q, want %q", cfg.Client.Model(), "nomic-embed-text")
		}
	})
}

// ============================================================================
// embedNote edge-case tests
// ============================================================================

func TestEmbedNoteEdgeCases(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)
	embedCfg := &EmbedConfig{
		Enabled: true,
		Client:  embeddings.NewClient("http://localhost:11434", "nomic-embed-text"),
	}

	t.Run("empty body returns zero", func(t *testing.T) {
		n, err := idx.embedNote("note1", "", embedCfg)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0 chunks, got %d", n)
		}
	})

	t.Run("nil embed config returns zero", func(t *testing.T) {
		n, err := idx.embedNote("note1", "some content", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if n != 0 {
			t.Errorf("expected 0 chunks, got %d", n)
		}
	})

	t.Run("embedding generation error with multiple chunks", func(t *testing.T) {
		// Use an OpenAI-compatible endpoint so GenerateBatch sends all texts in one request.
		client := embeddings.NewClient("http://localhost:11434/v1", "nomic-embed-text")
		client.SetHTTPClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				body := map[string]interface{}{
					"data": []map[string]interface{}{
						{"embedding": []float32{0.1, 0.2, 0.3}, "index": 0},
					},
				}
				b, _ := json.Marshal(body)
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(b)),
					Header:     make(http.Header),
				}, nil
			}),
		})
		cfg := &EmbedConfig{Enabled: true, Client: client}

		// Generate enough words to force more than one chunk.
		body := "# Heading\n\n" + strings.Repeat("This is a sentence with enough words to produce multiple chunks. ", 100)
		_, err := idx.embedNote("note1", body, cfg)
		if err == nil {
			t.Fatal("expected error for embedding generation, got nil")
		}
		if !strings.Contains(err.Error(), "embedding generation failed") {
			t.Errorf("unexpected error message: %q", err.Error())
		}
	})
}

// ============================================================================
// Index edge-case tests
// ============================================================================

func TestIndexEmbedDisabled(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/idea.md", "---\ntitle: My Idea\ntype: note\n---\n\nContent long enough to chunk.")

	result, err := idx.Index(IndexOptions{Embed: true, embedCfg: &EmbedConfig{Enabled: false}})
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	if result.Added != 1 {
		t.Errorf("expected Added=1, got %d", result.Added)
	}
	if result.ChunksAdded != 0 {
		t.Errorf("expected ChunksAdded=0, got %d", result.ChunksAdded)
	}
	if result.EmbedErrors != 0 {
		t.Errorf("expected EmbedErrors=0, got %d", result.EmbedErrors)
	}
}

func TestIndexRebuildWithEmbed(t *testing.T) {
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

	if _, err := idx.Index(IndexOptions{Embed: true, embedCfg: embedCfg}); err != nil {
		t.Fatalf("initial index failed: %v", err)
	}

	result, err := idx.Index(IndexOptions{Rebuild: true, Embed: true, embedCfg: embedCfg})
	if err != nil {
		t.Fatalf("Index failed: %v", err)
	}
	if result.Updated != 1 {
		t.Errorf("expected Updated=1, got %d", result.Updated)
	}
	if result.Skipped != 0 {
		t.Errorf("expected Skipped=0, got %d", result.Skipped)
	}

	var chunkCount int
	row := database.QueryRow("SELECT COUNT(*) FROM chunks")
	if err := row.Scan(&chunkCount); err != nil {
		t.Fatalf("failed to count chunks: %v", err)
	}
	if chunkCount == 0 {
		t.Error("expected chunks to be repopulated after rebuild")
	}
}

func TestIndexNonExistentPath(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	_, err := idx.Index(IndexOptions{Path: "does-not-exist"})
	if err == nil {
		t.Fatal("expected error for non-existent path, got nil")
	}
}

func TestIndexFileReadError(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	writeNote(t, vaultPath, "10-notes/locked.md", "---\ntitle: Locked\ntype: note\n---\n\nContent.")
	fullPath := filepath.Join(vaultPath, "10-notes/locked.md")
	if err := os.Chmod(fullPath, 0000); err != nil {
		t.Fatalf("failed to chmod file: %v", err)
	}
	defer os.Chmod(fullPath, 0644)

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

func TestIndexNonMarkdownSkipped(t *testing.T) {
	vaultPath, database, cleanup := setupTestVault(t)
	defer cleanup()
	idx := New(database, vaultPath)

	fullPath := filepath.Join(vaultPath, "readme.txt")
	if err := os.WriteFile(fullPath, []byte("not markdown"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	result := mustIndex(t, idx, IndexOptions{})
	if result.Scanned != 0 {
		t.Errorf("expected Scanned=0, got %d", result.Scanned)
	}
	if result.Added != 0 {
		t.Errorf("expected Added=0, got %d", result.Added)
	}
}

func TestFilepathToIDNested(t *testing.T) {
	id := filepathToID("10-notes/sub/my note.md")
	if id != "10-notes_sub_my note" {
		t.Errorf("expected '10-notes_sub_my note', got %q", id)
	}
}

func TestComputeHashEmpty(t *testing.T) {
	h := ComputeHash([]byte{})
	if len(h) != 64 {
		t.Errorf("expected sha256 hex length 64, got %d", len(h))
	}
}
