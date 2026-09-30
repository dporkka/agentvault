package agentstate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/indexer"
)

func TestCompileContextDeterministicWithProvenance(t *testing.T) {
	vaultPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vaultPath, ".agentvault"), 0755); err != nil {
		t.Fatal(err)
	}

	database, err := db.Open(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.RunMigrations(); err != nil {
		t.Fatal(err)
	}

	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(vaultPath, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	write("75-agents/coder.md", `---
id: agt_context_1
type: agent
title: Coding Agent
revision: 3
identity_ref: identity_1
memory_refs: [memory_1, missing_memory]
knowledge_scopes: [project:adacavo]
artifact_scopes: [repo]
conversation_scopes: [conv]
capability_refs: [github]
context_policy_ref: policy_1
created: 2026-09-30T10:00:00Z
updated: 2026-09-30T10:00:00Z
---

# Coding Agent

Build small, reviewable changes.
`)
	write("10-notes/identity.md", `---
id: identity_1
type: note
title: Identity
---
Prefer explicit evidence and small diffs.
`)
	write("10-notes/policy.md", `---
id: policy_1
type: note
title: Context Policy
---
Include durable memory before retrieved knowledge.
`)
	write("10-notes/memory.md", `---
id: memory_1
type: note
title: Durable Memory
---
Use PostgreSQL for durable shared state.
`)
	write("10-notes/knowledge.md", `---
id: knowledge_1
type: note
title: Retrieved Knowledge
---
Customer requires CSV exports.
`)
	write("10-notes/artifact.md", `---
id: artifact_1
type: note
title: Current Artifact
---
PR #81 changes agent-state contracts.
`)

	idx := indexer.New(database, vaultPath)
	if _, err := idx.Index(indexer.IndexOptions{}); err != nil {
		t.Fatal(err)
	}

	if _, err := database.Exec(`
		INSERT INTO conversations (id, title, created_at, updated_at)
		VALUES ('conv_1', 'Context session', '2026-09-30T10:00:00Z', '2026-09-30T10:02:00Z')
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := database.Exec(`
		INSERT INTO conversation_messages (conversation_id, role, content, created_at)
		VALUES
		  ('conv_1', 'user', 'Implement the compiler.', '2026-09-30T10:01:00Z'),
		  ('conv_1', 'assistant', 'I will keep it deterministic.', '2026-09-30T10:02:00Z')
	`); err != nil {
		t.Fatal(err)
	}

	req := ContextCompileRequest{
		AgentID:                "agt_context_1",
		Task:                   "Compile context for the next run.",
		ConversationID:         "conv_1",
		RetrievedNoteIDs:       []string{"knowledge_1"},
		ArtifactNoteIDs:        []string{"artifact_1"},
		MaxConversationMessages: 10,
	}
	first, err := CompileContext(database, vaultPath, req)
	if err != nil {
		t.Fatalf("CompileContext: %v", err)
	}
	second, err := CompileContext(database, vaultPath, req)
	if err != nil {
		t.Fatalf("CompileContext second call: %v", err)
	}

	if first.Hash == "" || !strings.HasPrefix(first.Hash, "sha256:") {
		t.Fatalf("expected sha256 hash, got %q", first.Hash)
	}
	if first.Hash != second.Hash {
		t.Fatalf("expected deterministic hash, got %s then %s", first.Hash, second.Hash)
	}
	if first.AgentID != "agt_context_1" || first.AgentRevision != 3 {
		t.Fatalf("unexpected agent identity: %#v", first)
	}

	wantKinds := []ContextSectionKind{
		ContextAgent,
		ContextIdentity,
		ContextPolicy,
		ContextMemory,
		ContextTask,
		ContextKnowledge,
		ContextConversation,
		ContextConversation,
		ContextArtifact,
	}
	if len(first.Sections) != len(wantKinds) {
		t.Fatalf("expected %d sections, got %d: %#v", len(wantKinds), len(first.Sections), first.Sections)
	}
	for i, want := range wantKinds {
		if first.Sections[i].Kind != want {
			t.Fatalf("section %d: expected kind %q, got %q", i, want, first.Sections[i].Kind)
		}
	}
	if first.Sections[1].SourceID != "identity_1" || first.Sections[3].SourceID != "memory_1" {
		t.Fatalf("expected note provenance, got %#v", first.Sections)
	}
	if len(first.Unresolved) != 1 || first.Unresolved[0].SourceID != "missing_memory" || first.Unresolved[0].Kind != ContextMemory {
		t.Fatalf("expected unresolved memory reference, got %#v", first.Unresolved)
	}
	if len(first.KnowledgeScopes) != 1 || first.KnowledgeScopes[0] != "project:adacavo" {
		t.Fatalf("unexpected knowledge scopes: %#v", first.KnowledgeScopes)
	}
	if len(first.CapabilityRefs) != 1 || first.CapabilityRefs[0] != "github" {
		t.Fatalf("unexpected capability refs: %#v", first.CapabilityRefs)
	}

	var stored string
	if err := database.QueryRow(`SELECT context_json FROM context_snapshots WHERE hash = ?`, first.Hash).Scan(&stored); err != nil {
		t.Fatalf("expected persisted context snapshot: %v", err)
	}
	if !strings.Contains(stored, "knowledge_1") || !strings.Contains(stored, "missing_memory") {
		t.Fatalf("persisted snapshot missing provenance: %s", stored)
	}

	changed := req
	changed.Task = "A different task."
	third, err := CompileContext(database, vaultPath, changed)
	if err != nil {
		t.Fatalf("CompileContext changed task: %v", err)
	}
	if third.Hash == first.Hash {
		t.Fatal("expected task change to change context hash")
	}
}

func TestCompileContextRejectsNonAgentManifest(t *testing.T) {
	vaultPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vaultPath, ".agentvault"), 0755); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(vaultPath)
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	if err := database.RunMigrations(); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(vaultPath, "10-notes", "plain.md")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`---
id: note_plain
type: note
title: Plain
---
Not an agent.
`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := indexer.New(database, vaultPath).Index(indexer.IndexOptions{}); err != nil {
		t.Fatal(err)
	}

	_, err = CompileContext(database, vaultPath, ContextCompileRequest{AgentID: "note_plain"})
	if err == nil || !strings.Contains(err.Error(), "not an agent manifest") {
		t.Fatalf("expected non-agent error, got %v", err)
	}
}
