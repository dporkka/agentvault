package indexer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/embeddings"
)

func setupBenchVault(b *testing.B, noteCount int) (string, *db.DB) {
	b.Helper()
	tmpDir := b.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0755); err != nil {
		b.Fatalf("failed to create .agentvault dir: %v", err)
	}
	database, err := db.Open(tmpDir)
	if err != nil {
		b.Fatalf("failed to open db: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		b.Fatalf("failed to run migrations: %v", err)
	}

	body := "This is a representative note body. " +
		"It contains enough text to be realistic without being huge. " +
		"We repeat a few sentences to add length and variety. " +
		"Search and indexing benchmarks need content that resembles real notes. " +
		"Vector search also benefits from documents with several sentences. " +
		"Each note gets the same body in these benchmarks for simplicity.\n\n" +
		"## Section\n\n" +
		"More content here. Another paragraph with words like database, search, embedding, and vault. " +
		"The quick brown fox jumps over the lazy dog. " +
		"Artificial intelligence helps organize knowledge. " +
		"Local-first tools keep data under user control.\n"

	for i := 0; i < noteCount; i++ {
		relPath := fmt.Sprintf("10-notes/note-%04d.md", i)
		fullPath := filepath.Join(tmpDir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			b.Fatalf("failed to create dir: %v", err)
		}
		content := fmt.Sprintf("---\nid: note-%04d\ntype: note\ntitle: Note %04d\n---\n\n%s", i, i, body)
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			b.Fatalf("failed to write note: %v", err)
		}
	}

	return tmpDir, database
}

func BenchmarkIndexSmallVault(b *testing.B) {
	vaultPath, database := setupBenchVault(b, 10)
	defer database.Close()

	idx := New(database, vaultPath)
	if _, err := idx.Index(IndexOptions{}); err != nil {
		b.Fatalf("initial index failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Index(IndexOptions{Force: true}); err != nil {
			b.Fatalf("index failed: %v", err)
		}
	}
}

func BenchmarkIndexMediumVault(b *testing.B) {
	vaultPath, database := setupBenchVault(b, 100)
	defer database.Close()

	idx := New(database, vaultPath)
	if _, err := idx.Index(IndexOptions{}); err != nil {
		b.Fatalf("initial index failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Index(IndexOptions{Force: true}); err != nil {
			b.Fatalf("index failed: %v", err)
		}
	}
}

func BenchmarkIndexWithEmbeddings(b *testing.B) {
	vaultPath, database := setupBenchVault(b, 10)
	defer database.Close()

	client := embeddings.NewClient("http://localhost:11434", "nomic-embed-text")
	client.SetHTTPClient(&http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			body := map[string]interface{}{
				"embedding": []float32{0.1, 0.2, 0.3, 0.4},
			}
			data, _ := json.Marshal(body)
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(data)),
				Header:     make(http.Header),
			}, nil
		}),
	})
	embedCfg := &EmbedConfig{Enabled: true, Client: client}

	idx := New(database, vaultPath)
	if _, err := idx.Index(IndexOptions{Embed: true, embedCfg: embedCfg}); err != nil {
		b.Fatalf("initial index failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := idx.Index(IndexOptions{Force: true, Embed: true, embedCfg: embedCfg}); err != nil {
			b.Fatalf("index with embeddings failed: %v", err)
		}
	}
}
