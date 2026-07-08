package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/embeddings"
	"github.com/agentvault/core/internal/indexer"
)

func setupSearchBenchVault(b *testing.B, noteCount int, withEmbeddings bool) (string, *db.DB, *Searcher) {
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

	body := "This is a representative note body used for search benchmarks. " +
		"It talks about databases, search, embeddings, and local-first knowledge tools. " +
		"The quick brown fox jumps over the lazy dog. " +
		"Artificial intelligence helps organize notes and decisions. " +
		"Vector search enables semantic retrieval beyond keywords.\n"

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

	idx := indexer.New(database, tmpDir)
	if _, err := idx.Index(indexer.IndexOptions{}); err != nil {
		b.Fatalf("index failed: %v", err)
	}

	searcher := New(database)
	if withEmbeddings {
		// Store deterministic embeddings for each note so vector/hybrid search
		// benchmarks do not require a live embedding endpoint.
		emb := []float32{0.1, 0.2, 0.3, 0.4}
		for i := 0; i < noteCount; i++ {
			noteID := fmt.Sprintf("note-%04d", i)
			chunkID := fmt.Sprintf("%s_chunk_0", noteID)
			if err := searcher.StoreChunkEmbedding(chunkID, noteID, 0, body, "bench", emb); err != nil {
				b.Fatalf("failed to store embedding: %v", err)
			}
		}

		client := embeddings.NewClient("http://localhost:11434", "nomic-embed-text")
		client.SetHTTPClient(&http.Client{
			Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				respBody := map[string]interface{}{
					"embedding": []float32{0.1, 0.2, 0.3, 0.4},
				}
				data, _ := json.Marshal(respBody)
				return &http.Response{
					StatusCode: 200,
					Body:       io.NopCloser(bytes.NewReader(data)),
					Header:     make(http.Header),
				}, nil
			}),
		})
		searcher.SetEmbedClient(client)
	}

	return tmpDir, database, searcher
}

func BenchmarkSearchFTS(b *testing.B) {
	_, database, searcher := setupSearchBenchVault(b, 100, false)
	defer database.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searcher.Search(Query{Q: "search embeddings", Limit: 20})
		if err != nil {
			b.Fatalf("search failed: %v", err)
		}
	}
}

func BenchmarkVectorSearch(b *testing.B) {
	_, database, searcher := setupSearchBenchVault(b, 100, true)
	defer database.Close()

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searcher.VectorSearch(ctx, "semantic retrieval", 20)
		if err != nil {
			b.Fatalf("vector search failed: %v", err)
		}
	}
}

func BenchmarkHybridSearch(b *testing.B) {
	_, database, searcher := setupSearchBenchVault(b, 100, true)
	defer database.Close()

	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := searcher.HybridSearch(ctx, VectorQuery{
			Query:        Query{Q: "search embeddings", Limit: 20},
			VectorSearch: true,
			QueryText:    "semantic retrieval",
			TopK:         20,
			HybridWeight: 0.5,
		})
		if err != nil {
			b.Fatalf("hybrid search failed: %v", err)
		}
	}
}

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}
