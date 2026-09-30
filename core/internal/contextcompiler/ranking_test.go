package contextcompiler

import (
	"math"
	"testing"
	"time"

	"github.com/agentvault/core/internal/contract"
)

func TestRankContextItemExplainsDeterministicSignals(t *testing.T) {
	confidence := 0.9
	collector := &candidateCollector{
		req: contract.CompileContextRequest{
			Task:      "renewal approval workflow",
			ObjectIDs: []string{"obj-renewal"},
			Explain:   true,
		},
		asOf: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
	}
	item := contract.ContextItem{
		Kind:      "object",
		ID:        "obj-renewal",
		Title:     "Renewal approval",
		Content:   "Approval workflow for contract renewal",
		Score:     0.80,
		ObjectIDs: []string{"obj-renewal"},
		Provenance: &contract.ContextProvenance{
			Confidence: confidence,
			ObservedAt: "2026-09-29T12:00:00Z",
		},
		Metadata: map[string]interface{}{
			"updatedAt": "2026-09-29T12:00:00Z",
		},
	}

	ranked := collector.rank(item)
	if ranked.Ranking == nil {
		t.Fatal("expected ranking explanation")
	}
	if ranked.Ranking.Algorithm != "deterministic-multisignal-v1" {
		t.Fatalf("unexpected algorithm %q", ranked.Ranking.Algorithm)
	}
	if len(ranked.Ranking.Components) < 4 {
		t.Fatalf("expected multiple ranking components, got %+v", ranked.Ranking.Components)
	}

	sum := 0.0
	foundExplicit := false
	foundLexical := false
	for _, component := range ranked.Ranking.Components {
		sum += component.Contribution
		if component.Signal == "explicitObject" && component.Value == 1 {
			foundExplicit = true
		}
		if component.Signal == "lexicalRelevance" && component.Value > 0 {
			foundLexical = true
		}
	}
	if !foundExplicit {
		t.Fatal("expected explicit object signal")
	}
	if !foundLexical {
		t.Fatal("expected lexical relevance signal")
	}
	if math.Abs(sum-ranked.Score) > 0.000001 {
		t.Fatalf("component contributions %f do not explain score %f", sum, ranked.Score)
	}
}

func TestRankContextItemDoesNotUseFutureRecency(t *testing.T) {
	collector := &candidateCollector{
		req:  contract.CompileContextRequest{Task: "deployment", Explain: true},
		asOf: time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC),
	}
	item := contract.ContextItem{
		Kind:    "note",
		ID:      "future-note",
		Title:   "Deployment",
		Content: "deployment instructions",
		Score:   0.8,
		Metadata: map[string]interface{}{
			"updatedAt": "2026-10-01T12:00:00Z",
		},
	}

	ranked := collector.rank(item)
	for _, component := range ranked.Ranking.Components {
		if component.Signal == "recency" && component.Value != 0 {
			t.Fatalf("future timestamp must not receive recency credit: %+v", component)
		}
	}
}
