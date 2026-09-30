package contextcompiler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/embeddings"
)

func TestCompileAddsScopedSemanticCandidatesMissingFromLexicalSearch(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "semantic-only.md", `---
id: note_semantic_only
type: note
title: Contingency Procedure
project: adacavo
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Revert the release using the previous stable artifact.
`)
	writeAndIndexNote(t, database, vault, "lexical.md", `---
id: note_semantic_lexical
type: note
title: Deployment Checklist
project: adacavo
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Deployment checklist for production verification.
`)
	writeAndIndexNote(t, database, vault, "semantic-forbidden.md", `---
id: note_semantic_forbidden
type: note
title: Contingency Procedure
project: other-project
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Revert the release using the previous stable artifact.
`)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embedding":[1,0,0,0]}`))
	}))
	defer server.Close()
	compiler.searcher.SetEmbedClient(embeddings.NewClient(server.URL, "test"))

	if err := compiler.searcher.StoreChunkEmbedding("semantic_only_chunk", "note_semantic_only", 0, "rollback instructions", "test", []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := compiler.searcher.StoreChunkEmbedding("semantic_lexical_chunk", "note_semantic_lexical", 0, "deployment checklist", "test", []float32{0, 1, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := compiler.searcher.StoreChunkEmbedding("semantic_forbidden_chunk", "note_semantic_forbidden", 0, "rollback instructions", "test", []float32{1, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}

	bundle, err := compiler.Compile(contract.CompileContextRequest{
		Task:        "rollback recovery guidance",
		Project:     "adacavo",
		TokenBudget: 1200,
		MaxItems:    10,
		AsOf:        "2099-01-01T00:00:00Z",
		Explain:     true,
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	foundSemanticOnly := false
	for _, item := range bundle.Items {
		if item.ID == "note_semantic_forbidden" {
			t.Fatalf("cross-project semantic candidate leaked into context: %+v", item)
		}
		if item.ID == "note_semantic_only" {
			foundSemanticOnly = true
			if item.Ranking == nil || item.Ranking.Algorithm != "deterministic-multisignal-v2-semantic" {
				t.Fatalf("semantic-only candidate missing semantic ranking explanation: %+v", item.Ranking)
			}
		}
	}
	if !foundSemanticOnly {
		t.Fatalf("semantic-only candidate was not recovered: %+v", bundle.Items)
	}
}
