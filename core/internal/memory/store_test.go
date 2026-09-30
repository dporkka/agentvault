package memory

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/agentvault/core/internal/db"
)

func setupMemoryStore(t *testing.T) (*db.DB, *Store) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".agentvault"), 0755); err != nil {
		t.Fatalf("create agentvault directory: %v", err)
	}
	database, err := db.Open(root)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("run migrations: %v", err)
	}
	return database, NewStore(database)
}

func seedMemoryNote(t *testing.T, database *db.DB, id, title string) {
	t.Helper()
	now := "2026-09-10T10:00:00Z"
	fileID := "file-" + id
	if _, err := database.Exec(`
		INSERT INTO files (id, path, content_hash, created_at, updated_at, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, fileID, id+".md", "hash-"+id, now, now, now); err != nil {
		t.Fatalf("seed file %s: %v", id, err)
	}
	if _, err := database.Exec(`
		INSERT INTO notes (id, file_id, title, type, status, project, created_at, updated_at, body)
		VALUES (?, ?, ?, 'note', 'active', 'test', ?, ?, ?)
	`, id, fileID, title, now, now, "body for "+id); err != nil {
		t.Fatalf("seed note %s: %v", id, err)
	}
}

func confidence(value float64) *float64 { return &value }

func TestMigration003AddsMemorySchema(t *testing.T) {
	database, _ := setupMemoryStore(t)

	var version int
	if err := database.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	if version < 3 {
		t.Fatalf("schema version = %d, want at least 3", version)
	}

	columns := map[string]bool{}
	rows, err := database.Query("PRAGMA table_info(notes)")
	if err != nil {
		t.Fatalf("inspect notes columns: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, columnType string
		var notNull int
		var defaultValue interface{}
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			t.Fatalf("scan column: %v", err)
		}
		columns[name] = true
	}
	for _, name := range []string{"workspace_id", "agent_id", "session_id", "memory_kind", "confidence", "provenance_json", "observed_at", "valid_from", "valid_to"} {
		if !columns[name] {
			t.Errorf("migration did not add notes.%s", name)
		}
	}
}

func TestStoreProjectAndGet(t *testing.T) {
	database, store := setupMemoryStore(t)
	seedMemoryNote(t, database, "old", "Old Fact")
	seedMemoryNote(t, database, "new", "New Fact")

	metadata := Metadata{
		NoteID: "new",
		Scope: Scope{
			WorkspaceID: "workspace-a",
			AgentID:     "agent-a",
			SessionID:   "session-a",
		},
		Kind:       KindFact,
		Confidence: confidence(0.92),
		Provenance: Provenance{
			SourceType: "conversation",
			SourceRef:  "conversation-42",
			Actor:      "user",
			Model:      "model-x",
			CapturedAt: "2026-09-10T09:30:00Z",
		},
		ObservedAt:         "2026-09-10T09:45:00Z",
		ValidFrom:          "2026-09-10T09:45:00Z",
		ValidTo:            "2026-10-10T09:45:00Z",
		Supersedes:         []string{"old"},
		SupersessionReason: "newer user statement",
	}
	if err := store.Project(context.Background(), metadata); err != nil {
		t.Fatalf("Project() error: %v", err)
	}

	record, err := store.Get(context.Background(), "new")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if record.Scope != metadata.Scope {
		t.Fatalf("scope = %+v, want %+v", record.Scope, metadata.Scope)
	}
	if record.Kind != KindFact {
		t.Fatalf("kind = %q, want fact", record.Kind)
	}
	if record.Confidence == nil || *record.Confidence != 0.92 {
		t.Fatalf("confidence = %v, want 0.92", record.Confidence)
	}
	if record.Provenance.SourceRef != "conversation-42" {
		t.Fatalf("provenance = %+v", record.Provenance)
	}
	if len(record.Supersedes) != 1 || record.Supersedes[0] != "old" {
		t.Fatalf("supersedes = %#v, want [old]", record.Supersedes)
	}
	if record.SupersessionReason != "newer user statement" {
		t.Fatalf("supersession reason = %q", record.SupersessionReason)
	}

	old, err := store.Get(context.Background(), "old")
	if err != nil {
		t.Fatalf("Get(old) error: %v", err)
	}
	if !old.Superseded {
		t.Fatal("old memory should report an incoming supersession")
	}
}

func TestStoreQueryScopesTemporalAndContextualSupersession(t *testing.T) {
	database, store := setupMemoryStore(t)
	for _, item := range []struct{ id, title string }{
		{"global-old", "Global Old"},
		{"workspace-new", "Workspace New"},
		{"workspace-b", "Workspace B"},
		{"session-a", "Session A"},
		{"expired", "Expired"},
	} {
		seedMemoryNote(t, database, item.id, item.title)
	}

	project := func(metadata Metadata) {
		t.Helper()
		if err := store.Project(context.Background(), metadata); err != nil {
			t.Fatalf("Project(%s): %v", metadata.NoteID, err)
		}
	}
	project(Metadata{NoteID: "global-old", Kind: KindFact, Confidence: confidence(0.7)})
	project(Metadata{
		NoteID:             "workspace-new",
		Scope:              Scope{WorkspaceID: "workspace-a"},
		Kind:               KindFact,
		Confidence:         confidence(0.9),
		Supersedes:         []string{"global-old"},
		SupersessionReason: "workspace-specific correction",
	})
	project(Metadata{
		NoteID:     "workspace-b",
		Scope:      Scope{WorkspaceID: "workspace-b"},
		Kind:       KindFact,
		Confidence: confidence(0.8),
	})
	project(Metadata{
		NoteID: "session-a",
		Scope: Scope{
			WorkspaceID: "workspace-a",
			AgentID:     "agent-a",
			SessionID:   "session-a",
		},
		Kind:       KindFact,
		Confidence: confidence(0.95),
	})
	project(Metadata{
		NoteID:     "expired",
		Scope:      Scope{WorkspaceID: "workspace-a"},
		Kind:       KindFact,
		Confidence: confidence(0.99),
		ValidTo:    "2026-09-10T11:00:00Z",
	})

	at := time.Date(2026, 9, 10, 12, 0, 0, 0, time.UTC)
	queryIDs := func(scope Scope, includeSuperseded bool) []string {
		t.Helper()
		records, err := store.Query(context.Background(), Query{
			Context:           scope,
			Kinds:             []Kind{KindFact},
			At:                &at,
			IncludeSuperseded: includeSuperseded,
			Limit:             20,
		})
		if err != nil {
			t.Fatalf("Query(%+v): %v", scope, err)
		}
		ids := make([]string, len(records))
		for i, record := range records {
			ids[i] = record.NoteID
		}
		sort.Strings(ids)
		return ids
	}

	workspaceA := Scope{WorkspaceID: "workspace-a", AgentID: "agent-a", SessionID: "session-a"}
	if got := queryIDs(workspaceA, false); !equalStrings(got, []string{"session-a", "workspace-new"}) {
		t.Fatalf("workspace A visible memories = %#v", got)
	}

	workspaceB := Scope{WorkspaceID: "workspace-b", AgentID: "agent-a", SessionID: "session-a"}
	if got := queryIDs(workspaceB, false); !equalStrings(got, []string{"global-old", "workspace-b"}) {
		t.Fatalf("workspace B visible memories = %#v; workspace-A supersession must not hide global memory here", got)
	}

	if got := queryIDs(workspaceA, true); !equalStrings(got, []string{"global-old", "session-a", "workspace-new"}) {
		t.Fatalf("include superseded visible memories = %#v", got)
	}
}

func TestStoreQueryMinConfidence(t *testing.T) {
	database, store := setupMemoryStore(t)
	seedMemoryNote(t, database, "low", "Low")
	seedMemoryNote(t, database, "high", "High")
	if err := store.Project(context.Background(), Metadata{NoteID: "low", Kind: KindFact, Confidence: confidence(0.4)}); err != nil {
		t.Fatal(err)
	}
	if err := store.Project(context.Background(), Metadata{NoteID: "high", Kind: KindFact, Confidence: confidence(0.9)}); err != nil {
		t.Fatal(err)
	}

	minimum := 0.8
	records, err := store.Query(context.Background(), Query{Kinds: []Kind{KindFact}, MinConfidence: &minimum})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 || records[0].NoteID != "high" {
		t.Fatalf("confidence-filtered records = %#v", records)
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
