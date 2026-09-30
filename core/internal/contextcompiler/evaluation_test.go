package contextcompiler

import (
	"reflect"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestContextCompilerGoldenEvaluation(t *testing.T) {
	compiler, store, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "eval-runbook.md", `---
id: note_eval_runbook
type: note
title: Deployment Verification Runbook
project: adacavo
created: 2026-09-20T10:00:00Z
updated: 2026-09-20T10:00:00Z
---
Deployment changes require a signed verification record and rollback check.
`)

	confidence := 0.98
	provenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_eval",
		SourceType: "file",
		SourceID:   "eval-runbook",
		AgentID:    "release-agent",
		Confidence: confidence,
		ObservedAt: "2026-09-20T10:00:00Z",
		Evidence: []contract.ProvenanceEvidence{{
			Source: "file",
			Path:   "10-notes/eval-runbook.md",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:           "mem_eval_required",
		MemoryClass:  "semantic",
		MemoryKind:   "constraint",
		ScopeType:    "project",
		ScopeID:      "adacavo",
		Content:      "Production deployments require a signed verification record.",
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:           "mem_eval_forbidden",
		MemoryClass:  "semantic",
		MemoryKind:   "constraint",
		ScopeType:    "project",
		ScopeID:      "other-project",
		Content:      "Secret other-project deployment procedure.",
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
	}); err != nil {
		t.Fatal(err)
	}

	request := contract.CompileContextRequest{
		Task:        "verify the production deployment",
		Project:     "adacavo",
		TokenBudget: 1200,
		MaxItems:    20,
		AsOf:        "2026-09-20T12:00:00Z",
	}

	first, err := compiler.Compile(request)
	if err != nil {
		t.Fatalf("first Compile: %v", err)
	}
	second, err := compiler.Compile(request)
	if err != nil {
		t.Fatalf("second Compile: %v", err)
	}

	if !reflect.DeepEqual(first, second) {
		t.Fatalf("fixed-input compilation is not deterministic:\nfirst=%+v\nsecond=%+v", first, second)
	}
	if first.EstimatedTokens > first.TokenBudget {
		t.Fatalf("compiled context exceeded budget: estimated=%d budget=%d", first.EstimatedTokens, first.TokenBudget)
	}

	items := make(map[string]contract.ContextItem, len(first.Items))
	for _, item := range first.Items {
		items[item.ID] = item
	}

	required, ok := items["mem_eval_required"]
	if !ok {
		t.Fatalf("required memory missing; got ids=%v", mapKeys(items))
	}
	if required.Provenance == nil || required.Provenance.ID != provenance.ID {
		t.Fatalf("required memory lost provenance: %+v", required)
	}
	if _, leaked := items["mem_eval_forbidden"]; leaked {
		t.Fatalf("cross-project memory leaked into compiled context: %+v", items["mem_eval_forbidden"])
	}
	if _, ok := items["note_eval_runbook"]; !ok {
		t.Fatalf("supporting runbook missing; got ids=%v", mapKeys(items))
	}
}
