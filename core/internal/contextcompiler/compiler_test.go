package contextcompiler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
	"github.com/agentvault/core/internal/knowledge"
	"github.com/agentvault/core/internal/search"
)

func setupCompiler(t *testing.T) (*Compiler, *knowledge.Store, *db.DB, string) {
	t.Helper()
	vault := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vault, ".agentvault"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(vault, "10-notes"), 0o755); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(vault)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		t.Fatal(err)
	}
	store := knowledge.New(database, vault)
	searcher := search.New(database)
	return New(searcher, store), store, database, vault
}

func writeAndIndexNote(t *testing.T, database *db.DB, vault, name, content string) {
	t.Helper()
	path := filepath.Join(vault, "10-notes", name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := indexer.New(database, vault).Index(indexer.IndexOptions{}); err != nil {
		t.Fatalf("index note: %v", err)
	}
}

func TestCompileBuildsCurrentEvidenceBackedContext(t *testing.T) {
	compiler, store, database, vault := setupCompiler(t)
	defer database.Close()

	writeAndIndexNote(t, database, vault, "renewals.md", `---
id: note_renewals
type: note
title: Contract Renewal Workflow
project: adacavo
status: active
created: 2026-09-01T00:00:00Z
updated: 2026-09-10T00:00:00Z
---
Contract renewals require finance approval before customer signature. The renewal workflow must preserve an audit trail.
`)

	confidence := 0.96
	provenance, err := store.CreateProvenance(contract.ProvenanceRecord{
		ID:         "prov_context",
		SourceType: "file",
		SourceID:   "renewal-spec",
		AgentID:    "architect",
		Confidence: confidence,
		ObservedAt: "2026-09-09T12:00:00Z",
		Evidence: []contract.ProvenanceEvidence{{
			Source: "file",
			Path:   "10-notes/renewals.md",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	project, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:           "obj_adacavo",
		Type:         "project",
		Title:        "Adacavo",
		Project:      "adacavo",
		ProvenanceID: provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	decision, err := store.UpsertObject(contract.UpsertKnowledgeObjectRequest{
		ID:            "obj_renewal_decision",
		Type:          "decision",
		Title:         "Finance approves contract renewals",
		Project:       "adacavo",
		CanonicalPath: "30-decisions/renewal-approval.md",
		ProvenanceID:  provenance.ID,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRelation(contract.CreateObjectRelationRequest{
		ID:           "rel_current",
		FromObjectID: decision.ID,
		ToObjectID:   project.ID,
		RelationType: "affects",
		ValidFrom:    "2026-01-01",
		Confidence:   &confidence,
		ProvenanceID: provenance.ID,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateRelation(contract.CreateObjectRelationRequest{
		ID:           "rel_expired",
		FromObjectID: project.ID,
		ToObjectID:   decision.ID,
		RelationType: "supersedes",
		ValidTo:      "2026-01-01",
		Confidence:   &confidence,
		ProvenanceID: provenance.ID,
	}); err != nil {
		t.Fatal(err)
	}

	oldMemory, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:           "mem_old",
		MemoryType:   "semantic",
		ScopeType:    "project",
		ScopeID:      "adacavo",
		Content:      "Sales approves contract renewals.",
		ObjectID:     decision.ID,
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RecordMemory(contract.CreateMemoryRequest{
		ID:           "mem_current",
		MemoryType:   "semantic",
		ScopeType:    "project",
		ScopeID:      "adacavo",
		Content:      "Finance approves contract renewals before signature.",
		ObjectID:     decision.ID,
		ProvenanceID: provenance.ID,
		Confidence:   &confidence,
		SupersedesID: oldMemory.ID,
	}); err != nil {
		t.Fatal(err)
	}

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		ID:        "session_context",
		AgentID:   "backend-engineer",
		Project:   "adacavo",
		Objective: "Implement the contract renewal approval workflow",
		Branch:    "feat/renewals",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendSessionEvent(session.ID, contract.AppendSessionEventRequest{
		ID:        "event_context",
		EventType: "decision",
		Payload: map[string]interface{}{
			"summary": "Preserve finance approval as a domain invariant",
		},
		ProvenanceID: provenance.ID,
	}); err != nil {
		t.Fatal(err)
	}

	bundle, err := compiler.Compile(contract.CompileContextRequest{
		Task:        "Implement contract renewal finance approval workflow",
		Project:     "adacavo",
		AgentID:     "backend-engineer",
		SessionID:   session.ID,
		ObjectIDs:   []string{decision.ID},
		TokenBudget: 2000,
		AsOf:        "2026-09-10T12:00:00Z",
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if bundle.EstimatedTokens > bundle.TokenBudget {
		t.Fatalf("compiled context exceeded budget: %+v", bundle)
	}
	if bundle.AsOf != "2026-09-10T12:00:00Z" {
		t.Fatalf("unexpected asOf %q", bundle.AsOf)
	}

	ids := make(map[string]contract.ContextItem)
	for _, item := range bundle.Items {
		ids[item.ID] = item
	}
	for _, required := range []string{"session_context", "event_context", "mem_current", "obj_renewal_decision", "rel_current", "note_renewals"} {
		if _, ok := ids[required]; !ok {
			t.Errorf("missing expected context item %s; got ids=%v", required, mapKeys(ids))
		}
	}
	if _, ok := ids["mem_old"]; ok {
		t.Error("superseded memory must not be compiled as current context")
	}
	if _, ok := ids["rel_expired"]; ok {
		t.Error("expired relation must not be compiled for current asOf")
	}
	if item := ids["mem_current"]; item.Provenance == nil || item.Provenance.ID != provenance.ID || item.Provenance.SourceType != "file" {
		t.Fatalf("memory provenance was not preserved: %+v", item)
	}
}

func TestCompileTruncatesToSmallBudget(t *testing.T) {
	compiler, _, database, vault := setupCompiler(t)
	defer database.Close()

	longBody := strings.Repeat("renewal approval workflow evidence and implementation detail. ", 600)
	writeAndIndexNote(t, database, vault, "large.md", `---
id: note_large
type: note
title: Large Renewal Note
project: adacavo
created: 2026-09-01T00:00:00Z
updated: 2026-09-10T00:00:00Z
---
`+longBody)

	bundle, err := compiler.Compile(contract.CompileContextRequest{
		Task:        "renewal approval workflow",
		Project:     "adacavo",
		TokenBudget: 256,
		MaxItems:    5,
		AsOf:        time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC).Format(time.RFC3339),
	})
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	if bundle.EstimatedTokens > 256 {
		t.Fatalf("expected <=256 estimated tokens, got %d", bundle.EstimatedTokens)
	}
	if !bundle.Truncated {
		t.Fatal("expected context bundle to report truncation")
	}
	if len(bundle.Items) == 0 {
		t.Fatal("expected at least one fitted context item")
	}
	for _, item := range bundle.Items {
		if item.EstimatedTokens <= 0 {
			t.Fatalf("context item has invalid token estimate: %+v", item)
		}
	}
}

func TestCompileRejectsSessionScopeMismatch(t *testing.T) {
	compiler, store, database, _ := setupCompiler(t)
	defer database.Close()

	session, err := store.StartSession(contract.StartAgentSessionRequest{
		AgentID:   "agent-a",
		Project:   "project-a",
		Objective: "Do work",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = compiler.Compile(contract.CompileContextRequest{
		Task:      "Do work",
		Project:   "project-b",
		SessionID: session.ID,
	})
	if err == nil {
		t.Fatal("expected project/session mismatch to fail")
	}
}

func mapKeys(values map[string]contract.ContextItem) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
