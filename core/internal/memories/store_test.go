package memories

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/agents"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/events"
)

func setupMemoryStore(t *testing.T) (*Store, *db.DB, agents.Agent, events.Event) {
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

	agent, err := agents.NewStore(database).Create(context.Background(), agents.CreateInput{
		Name: "researcher",
	})
	if err != nil {
		database.Close()
		t.Fatalf("create agent: %v", err)
	}

	sourceEvent, err := events.NewStore(database).Append(context.Background(), events.AppendInput{
		Type:      "observation.recorded",
		ActorType: "agent",
		ActorID:   agent.ID,
		Payload: map[string]interface{}{
			"text": "SQLite is the durable local index.",
		},
	})
	if err != nil {
		database.Close()
		t.Fatalf("append source event: %v", err)
	}

	return NewStore(database), database, agent, sourceEvent
}

func TestCreateCandidatePersistsTypedMemoryWithProvenance(t *testing.T) {
	store, database, agent, sourceEvent := setupMemoryStore(t)
	defer database.Close()

	created, err := store.CreateCandidate(context.Background(), CreateCandidateInput{
		AgentID:       agent.ID,
		Kind:          KindSemantic,
		Content:       "AgentVault uses SQLite as a rebuildable local index.",
		Confidence:    0.92,
		Salience:      0.81,
		ScopeType:     "project",
		ScopeID:       "agentvault",
		SourceEventID: sourceEvent.ID,
		Metadata:      map[string]interface{}{"extractor": "test"},
	})
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	if !strings.HasPrefix(created.ID, "mem_") {
		t.Fatalf("expected mem_ id prefix, got %q", created.ID)
	}
	if created.Kind != KindSemantic || created.Status != StatusCandidate {
		t.Fatalf("unexpected memory: %+v", created)
	}
	if created.SourceEventID != sourceEvent.ID {
		t.Fatalf("source event = %q, want %q", created.SourceEventID, sourceEvent.ID)
	}
	if created.PromotedAt != nil {
		t.Fatalf("candidate should not have promoted_at: %v", created.PromotedAt)
	}

	got, err := store.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get memory: %v", err)
	}
	if got.Content != created.Content || got.Confidence != 0.92 || got.Salience != 0.81 {
		t.Fatalf("persisted memory mismatch: %+v", got)
	}
	if got.Metadata["extractor"] != "test" {
		t.Fatalf("metadata = %#v", got.Metadata)
	}

	var auditCount int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM events WHERE event_type = ? AND subject_type = ? AND subject_id = ? AND parent_event_id = ?",
		"memory.candidate.created", "memory", created.ID, sourceEvent.ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("query candidate audit event: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected one candidate audit event, got %d", auditCount)
	}
}

func TestCreateCandidateRequiresSourceEvent(t *testing.T) {
	store, database, agent, _ := setupMemoryStore(t)
	defer database.Close()

	_, err := store.CreateCandidate(context.Background(), CreateCandidateInput{
		AgentID:    agent.ID,
		Kind:       KindEpisodic,
		Content:    "A conversation happened.",
		Confidence: 0.7,
		Salience:   0.5,
	})
	if err == nil {
		t.Fatal("expected source event to be required")
	}
}

func TestCreateCandidateValidatesKindAndScores(t *testing.T) {
	store, database, agent, sourceEvent := setupMemoryStore(t)
	defer database.Close()

	cases := []CreateCandidateInput{
		{AgentID: agent.ID, Kind: "unknown", Content: "x", Confidence: 0.5, Salience: 0.5, SourceEventID: sourceEvent.ID},
		{AgentID: agent.ID, Kind: KindSemantic, Content: "x", Confidence: 1.1, Salience: 0.5, SourceEventID: sourceEvent.ID},
		{AgentID: agent.ID, Kind: KindSemantic, Content: "x", Confidence: 0.5, Salience: -0.1, SourceEventID: sourceEvent.ID},
		{AgentID: agent.ID, Kind: KindSemantic, Content: "   ", Confidence: 0.5, Salience: 0.5, SourceEventID: sourceEvent.ID},
	}

	for i, input := range cases {
		if _, err := store.CreateCandidate(context.Background(), input); err == nil {
			t.Fatalf("case %d: expected validation error", i)
		}
	}
}

func TestPromoteCandidateMakesMemoryDurableAndAuditable(t *testing.T) {
	store, database, agent, sourceEvent := setupMemoryStore(t)
	defer database.Close()

	candidate, err := store.CreateCandidate(context.Background(), CreateCandidateInput{
		AgentID:       agent.ID,
		Kind:          KindProcedural,
		Content:       "Run focused tests before the full suite.",
		Confidence:    0.88,
		Salience:      0.9,
		SourceEventID: sourceEvent.ID,
	})
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	promoted, err := store.Promote(context.Background(), candidate.ID, PromoteInput{
		ActorType: "agent",
		ActorID:   agent.ID,
		Reason:    "Validated repeatedly.",
	})
	if err != nil {
		t.Fatalf("promote candidate: %v", err)
	}
	if promoted.Status != StatusDurable || promoted.PromotedAt == nil {
		t.Fatalf("memory was not promoted: %+v", promoted)
	}

	var auditCount int
	if err := database.QueryRow(
		"SELECT COUNT(*) FROM events WHERE event_type = ? AND subject_type = ? AND subject_id = ?",
		"memory.promoted", "memory", candidate.ID,
	).Scan(&auditCount); err != nil {
		t.Fatalf("query promotion audit event: %v", err)
	}
	if auditCount != 1 {
		t.Fatalf("expected one promotion event, got %d", auditCount)
	}

	if _, err := store.Promote(context.Background(), candidate.ID, PromoteInput{
		ActorType: "agent",
		ActorID:   agent.ID,
	}); err == nil {
		t.Fatal("expected second promotion to be rejected")
	}
}

func TestPromotionRollsBackWhenAuditEventFails(t *testing.T) {
	store, database, agent, sourceEvent := setupMemoryStore(t)
	defer database.Close()

	candidate, err := store.CreateCandidate(context.Background(), CreateCandidateInput{
		AgentID:       agent.ID,
		Kind:          KindSemantic,
		Content:       "Promotion must be atomic with its audit event.",
		Confidence:    0.95,
		Salience:      0.95,
		SourceEventID: sourceEvent.ID,
	})
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}

	if _, err := database.Exec(`
		CREATE TRIGGER fail_memory_promotion_event
		BEFORE INSERT ON events
		WHEN NEW.event_type = 'memory.promoted'
		BEGIN
			SELECT RAISE(ABORT, 'forced audit failure');
		END;
	`); err != nil {
		t.Fatalf("create failure trigger: %v", err)
	}

	if _, err := store.Promote(context.Background(), candidate.ID, PromoteInput{
		ActorType: "agent",
		ActorID:   agent.ID,
	}); err == nil {
		t.Fatal("expected promotion to fail when audit insert fails")
	}

	got, err := store.Get(context.Background(), candidate.ID)
	if err != nil {
		t.Fatalf("get memory after failed promotion: %v", err)
	}
	if got.Status != StatusCandidate || got.PromotedAt != nil {
		t.Fatalf("failed promotion mutated memory: %+v", got)
	}
}
