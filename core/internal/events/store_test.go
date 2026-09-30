package events

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agentvault/core/internal/db"
)

func setupEventStore(t *testing.T) (*Store, *db.DB) {
	t.Helper()
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, ".agentvault"), 0o755); err != nil {
		t.Fatalf("create .agentvault: %v", err)
	}
	database, err := db.Open(tmpDir)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := database.RunMigrations(); err != nil {
		database.Close()
		t.Fatalf("run migrations: %v", err)
	}
	return NewStore(database), database
}

func TestAppendPersistsEventWithProvenance(t *testing.T) {
	store, database := setupEventStore(t)
	defer database.Close()

	if _, err := database.Exec(
		"INSERT INTO conversations (id, title, created_at, updated_at) VALUES (?, ?, ?, ?)",
		"conv_123", "Test conversation", time.Now().UTC().Format(time.RFC3339Nano), time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("seed conversation: %v", err)
	}
	if _, err := database.Exec(
		"INSERT INTO agent_runs (id, agent_name, task, created_at) VALUES (?, ?, ?, ?)",
		"run_123", "test-agent", "test task", time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("seed run: %v", err)
	}

	occurred := time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)
	event, err := store.Append(context.Background(), AppendInput{
		Type:           "agent.run.logged",
		ActorType:      "agent",
		ActorID:        "agt_123",
		SubjectType:    "agent_run",
		SubjectID:      "run_123",
		ScopeType:      "project",
		ScopeID:        "agentvault",
		RunID:          "run_123",
		ConversationID: "conv_123",
		SourceID:       "src_123",
		OccurredAt:     occurred,
		Payload:        map[string]interface{}{"task": "review code"},
		Metadata:       map[string]interface{}{"origin": "mcp"},
	})
	if err != nil {
		t.Fatalf("append event: %v", err)
	}
	if !strings.HasPrefix(event.ID, "evt_") {
		t.Fatalf("expected evt_ id prefix, got %q", event.ID)
	}
	if !event.OccurredAt.Equal(occurred) {
		t.Fatalf("occurred_at = %s, want %s", event.OccurredAt, occurred)
	}
	if event.RecordedAt.IsZero() {
		t.Fatal("recorded_at should be set")
	}

	got, err := store.Get(context.Background(), event.ID)
	if err != nil {
		t.Fatalf("get event: %v", err)
	}
	if got.ActorID != "agt_123" || got.RunID != "run_123" || got.SourceID != "src_123" {
		t.Fatalf("provenance was not preserved: %+v", got)
	}
	if got.Payload["task"] != "review code" {
		t.Errorf("payload = %#v", got.Payload)
	}
}

func TestEventsAreImmutableAtDatabaseBoundary(t *testing.T) {
	store, database := setupEventStore(t)
	defer database.Close()

	event, err := store.Append(context.Background(), AppendInput{Type: "observation.recorded"})
	if err != nil {
		t.Fatalf("append event: %v", err)
	}

	if _, err := database.Exec("UPDATE events SET event_type = ? WHERE id = ?", "changed", event.ID); err == nil {
		t.Fatal("expected UPDATE on events to be rejected")
	}
	if _, err := database.Exec("DELETE FROM events WHERE id = ?", event.ID); err == nil {
		t.Fatal("expected DELETE on events to be rejected")
	}

	var eventType string
	if err := database.QueryRow("SELECT event_type FROM events WHERE id = ?", event.ID).Scan(&eventType); err != nil {
		t.Fatalf("read event after rejected mutations: %v", err)
	}
	if eventType != "observation.recorded" {
		t.Fatalf("event mutated despite append-only contract: %q", eventType)
	}
}

func TestAppendRejectsEmptyType(t *testing.T) {
	store, database := setupEventStore(t)
	defer database.Close()

	if _, err := store.Append(context.Background(), AppendInput{}); err == nil {
		t.Fatal("expected empty event type to be rejected")
	}
}

func TestGetMissingEventReturnsNotFound(t *testing.T) {
	store, database := setupEventStore(t)
	defer database.Close()

	_, err := store.Get(context.Background(), "evt_missing")
	if err == nil {
		t.Fatal("expected missing event error")
	}
	if err != sql.ErrNoRows {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
}
