package agents

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/db"
)

func setupAgentStore(t *testing.T) (*Store, *db.DB) {
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

func TestCreatePersistsCanonicalAgent(t *testing.T) {
	store, database := setupAgentStore(t)
	defer database.Close()

	created, err := store.Create(context.Background(), CreateInput{
		Name:         "researcher",
		DisplayName:  "Research Agent",
		Description:  "Investigates technical questions",
		Instructions: "Prefer primary sources.",
		Permissions:  []string{"read", "annotate"},
		Metadata:     map[string]interface{}{"owner": "local"},
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	if !strings.HasPrefix(created.ID, "agt_") {
		t.Fatalf("expected agt_ id prefix, got %q", created.ID)
	}
	if created.Name != "researcher" || created.Status != StatusActive {
		t.Fatalf("unexpected created agent: %+v", created)
	}

	got, err := store.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("get agent: %v", err)
	}
	if got.DisplayName != "Research Agent" {
		t.Errorf("display name = %q", got.DisplayName)
	}
	if len(got.Permissions) != 2 || got.Permissions[0] != "read" || got.Permissions[1] != "annotate" {
		t.Errorf("permissions = %#v", got.Permissions)
	}
	if got.Metadata["owner"] != "local" {
		t.Errorf("metadata = %#v", got.Metadata)
	}
}

func TestEnsureByNameIsIdempotent(t *testing.T) {
	store, database := setupAgentStore(t)
	defer database.Close()

	first, err := store.EnsureByName(context.Background(), "codex")
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	second, err := store.EnsureByName(context.Background(), "codex")
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}

	if first.ID != second.ID {
		t.Fatalf("expected stable agent id, got %q then %q", first.ID, second.ID)
	}
	agents, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("list agents: %v", err)
	}
	if len(agents) != 1 {
		t.Fatalf("expected one canonical agent, got %d", len(agents))
	}
}

func TestCreateRejectsEmptyName(t *testing.T) {
	store, database := setupAgentStore(t)
	defer database.Close()

	if _, err := store.Create(context.Background(), CreateInput{}); err == nil {
		t.Fatal("expected empty agent name to be rejected")
	}
}
