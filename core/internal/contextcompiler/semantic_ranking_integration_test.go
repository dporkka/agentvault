package contextcompiler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/embeddings"
)

func TestCompileUsesSemanticSimilarityToRerankScopedNoteCandidates(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "semantic-a.md", `---
id: note_semantic_a
type: note
title: Release Procedure Alpha
project: adacavo
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Release deployment verification procedure.
`)
	writeAndIndexNote(t, database, vault, "semantic-b.md", `---
id: note_semantic_b
type: note
title: Release Procedure Beta
project: adacavo
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Release deployment verification procedure.
`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embedding":[1,0,0,0]}`))
	}))
	defer server.Close()
	compiler.searcher.SetEmbedClient(embeddings.NewClient(server.URL, "test"))

	if err := compiler.searcher.StoreChunkEmbedding("semantic_chunk_a", "note_semantic_a", 0, "alpha", "test", []float32{0, 1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := compiler.searcher.StoreChunkEmbedding("semantic_chunk_b", "note_semantic_b", 0, "beta", "test", []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}

	bundle, err := compiler.Compile(contract.CompileContextRequest{
		Task:        "release deployment verification procedure",
		Project:     "adacavo",
		TokenBudget: 1200,
		MaxItems:    10,
		AsOf:        "2099-01-01T00:00:00Z",
		Explain:     true,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	var alpha, beta *contract.ContextItem
	for i := range bundle.Items {
		item := &bundle.Items[i]
		switch item.ID {
		case "note_semantic_a":
			alpha = item
		case "note_semantic_b":
			beta = item
		}
	}
	if alpha == nil || beta == nil {
		t.Fatalf("expected both scoped note candidates; got %+v", bundle.Items)
	}
	if beta.Score <= alpha.Score {
		t.Fatalf("semantic match should outrank orthogonal note: beta=%f alpha=%f", beta.Score, alpha.Score)
	}
	if beta.Ranking == nil || beta.Ranking.Algorithm != "deterministic-multisignal-v2-semantic" {
		t.Fatalf("expected semantic ranking explanation: %+v", beta.Ranking)
	}

	found := false
	for _, component := range beta.Ranking.Components {
		if component.Signal == "semanticSimilarity" {
			found = true
			if component.Value < 0.99 {
				t.Fatalf("expected near-perfect semantic score, got %+v", component)
			}
		}
	}
	if !found {
		t.Fatal("semantic ranking explanation omitted semanticSimilarity")
	}

	// Both candidates came from the already project-scoped FTS result set.
	for _, item := range bundle.Items {
		if item.Kind == "note" && strings.TrimSpace(item.Metadata["project"].(string)) != "adacavo" {
			t.Fatalf("semantic reranking introduced an out-of-scope note: %+v", item)
		}
	}
}
