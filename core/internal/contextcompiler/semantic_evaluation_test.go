package contextcompiler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/embeddings"
)

func TestSemanticContextEvaluationSuite(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "semantic-lexical-distractor.md", `---
id: note_semantic_lexical_distractor
type: note
title: Release Deployment Verification Procedure Checklist
project: adacavo
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Release deployment verification procedure release deployment verification procedure checklist.
`)
	writeAndIndexNote(t, database, vault, "semantic-target.md", `---
id: note_semantic_target
type: note
title: Release Handoff
project: adacavo
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Deployment verification handoff.
`)
	writeAndIndexNote(t, database, vault, "semantic-other-project.md", `---
id: note_semantic_other_project
type: note
title: Release Deployment Verification Procedure
project: other-project
created: 2026-09-20T00:00:00Z
updated: 2026-09-29T00:00:00Z
---
Release deployment verification procedure.
`)

	if err := compiler.searcher.StoreChunkEmbedding(
		"semantic_eval_distractor",
		"note_semantic_lexical_distractor",
		0,
		"lexical distractor",
		"test",
		[]float32{0, 1, 0, 0},
	); err != nil {
		t.Fatal(err)
	}
	if err := compiler.searcher.StoreChunkEmbedding(
		"semantic_eval_target",
		"note_semantic_target",
		0,
		"semantic target",
		"test",
		[]float32{1, 0, 0, 0},
	); err != nil {
		t.Fatal(err)
	}
	if err := compiler.searcher.StoreChunkEmbedding(
		"semantic_eval_other",
		"note_semantic_other_project",
		0,
		"cross project exact match",
		"test",
		[]float32{1, 0, 0, 0},
	); err != nil {
		t.Fatal(err)
	}

	request := contract.CompileContextRequest{
		Task:        "release deployment verification procedure",
		Project:     "adacavo",
		TokenBudget: 1200,
		MaxItems:    10,
		AsOf:        "2099-01-01T00:00:00Z",
		Explain:     true,
	}

	// Baseline: embeddings exist, but no embedding client is configured. This
	// intentionally exercises the exact deterministic-v1 fallback.
	baselineBundle, err := compiler.Compile(request)
	if err != nil {
		t.Fatalf("baseline Compile: %v", err)
	}
	baseline := EvaluateRetrieval([]RetrievalEvaluationCase{{
		Name:   "lexical baseline",
		Bundle: baselineBundle,
		Expect: RetrievalExpectation{
			RequiredIDs:  []string{"note_semantic_target", "note_semantic_lexical_distractor"},
			ForbiddenIDs: []string{"note_semantic_other_project"},
			PreferredBefore: []RankingPreference{{
				HigherID: "note_semantic_target",
				LowerID:  "note_semantic_lexical_distractor",
			}},
		},
	}})
	if baseline.Cases[0].Recall != 1 || baseline.TotalLeakage != 0 {
		t.Fatalf("baseline must preserve recall/isolation before semantic reranking: %+v", baseline)
	}
	if baseline.MeanPreferenceAccuracy != 0 {
		t.Fatalf("fixture must expose a lexical ordering error so semantic gain is measurable: %+v", baseline.Cases[0])
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embedding":[1,0,0,0]}`))
	}))
	defer server.Close()
	compiler.searcher.SetEmbedClient(embeddings.NewClient(server.URL, "test"))

	semanticBundle, err := compiler.Compile(request)
	if err != nil {
		t.Fatalf("semantic Compile: %v", err)
	}
	semantic := EvaluateRetrieval([]RetrievalEvaluationCase{{
		Name:   "semantic correction",
		Bundle: semanticBundle,
		Expect: RetrievalExpectation{
			RequiredIDs:  []string{"note_semantic_target", "note_semantic_lexical_distractor"},
			ForbiddenIDs: []string{"note_semantic_other_project"},
			PreferredBefore: []RankingPreference{{
				HigherID: "note_semantic_target",
				LowerID:  "note_semantic_lexical_distractor",
			}},
		},
	}})
	if !semantic.Passed {
		t.Fatalf("semantic evaluation should correct ordering without leakage: %+v", semantic)
	}
	if semantic.MacroRecall != 1 || semantic.TotalLeakage != 0 || semantic.MeanPreferenceAccuracy != 1 {
		t.Fatalf("semantic reranking regressed recall/isolation/preference accuracy: %+v", semantic)
	}
	if semantic.MeanRequiredPer1KTokens <= 0 {
		t.Fatalf("expected non-zero retrieval density: %+v", semantic)
	}

	t.Logf(
		"semantic eval: baseline_preference_accuracy=%.3f semantic_preference_accuracy=%.3f recall=%.3f leakage=%d required_per_1k=%.3f",
		baseline.MeanPreferenceAccuracy,
		semantic.MeanPreferenceAccuracy,
		semantic.MacroRecall,
		semantic.TotalLeakage,
		semantic.MeanRequiredPer1KTokens,
	)
}
