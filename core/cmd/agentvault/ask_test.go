package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/config"
	"github.com/agentvault/core/internal/rag"
)

func TestRunAskEmptyVaultMockProvider(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	cfg := config.DefaultConfig(vp)
	cfg.AI = &config.AIConfig{Provider: "mock"}
	if err := config.Save(vp, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	askProviderFlag = ""
	askModelFlag = ""
	askCommit = false

	cmd := cobraCommandArgs([]string{"what is this?"})
	if err := runAsk(cmd, []string{"what is this?"}); err != nil {
		t.Fatalf("runAsk returned error: %v", err)
	}
}

func TestRunAskProviderFlagOverride(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	// No AI config saved; the --provider flag should select the mock provider.
	askProviderFlag = "mock"
	askModelFlag = ""
	askCommit = false

	cmd := cobraCommandArgs([]string{"hello"})
	if err := runAsk(cmd, []string{"hello"}); err != nil {
		t.Fatalf("runAsk returned error: %v", err)
	}
}

func TestRunAskWithIndexedNote(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	cfg := config.DefaultConfig(vp)
	cfg.AI = &config.AIConfig{Provider: "mock"}
	if err := config.Save(vp, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	indexNote(t, database, vp, "10-notes/askable.md", "---\nid: ask_001\ntype: note\ntitle: Askable Note\n---\n\n# Askable Note\n\nAgentVault is a local-first knowledge system.")

	askProviderFlag = ""
	askModelFlag = ""
	askCommit = false

	cmd := cobraCommandArgs([]string{"What is AgentVault?"})
	if err := runAsk(cmd, []string{"What is AgentVault?"}); err != nil {
		t.Fatalf("runAsk returned error: %v", err)
	}
}

func TestRunAskCommitNotGitRepo(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	cfg := config.DefaultConfig(vp)
	cfg.AI = &config.AIConfig{Provider: "mock"}
	if err := config.Save(vp, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	askProviderFlag = ""
	askModelFlag = ""
	askCommit = true

	cmd := cobraCommandArgs([]string{"question"})
	if err := runAsk(cmd, []string{"question"}); err != nil {
		t.Fatalf("runAsk returned error: %v", err)
	}
}

func TestRunAskCommitWithGitRepo(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	initCmd := cobraCommandArgs([]string{})
	if err := runGitInit(initCmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	cfg := config.DefaultConfig(vp)
	cfg.AI = &config.AIConfig{Provider: "mock"}
	if err := config.Save(vp, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Create a change so the commit has something to stage.
	if err := os.WriteFile(filepath.Join(vp, "change.md"), []byte("change"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	askProviderFlag = ""
	askModelFlag = ""
	askCommit = true

	cmd := cobraCommandArgs([]string{"question with commit"})
	if err := runAsk(cmd, []string{"question with commit"}); err != nil {
		t.Fatalf("runAsk returned error: %v", err)
	}
}

func TestRunAskModelFlagOverride(t *testing.T) {
	vp, database := setupTestVaultWithDB(t)
	defer database.Close()
	vaultPath = vp

	cfg := config.DefaultConfig(vp)
	cfg.AI = &config.AIConfig{Provider: "mock", ChatModel: "old-model"}
	if err := config.Save(vp, cfg); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	askProviderFlag = ""
	askModelFlag = "new-model"
	askCommit = false

	cmd := cobraCommandArgs([]string{"test model override"})
	if err := runAsk(cmd, []string{"test model override"}); err != nil {
		t.Fatalf("runAsk returned error: %v", err)
	}
}

func TestPrintAnswer(t *testing.T) {
	// Ensure printAnswer does not panic with a fully populated answer.
	answer := &rag.Answer{
		Answer:           "The answer.",
		Sources:          []rag.Source{{Path: "10-notes/x.md", Title: "X", Excerpt: "excerpt"}},
		Confidence:       "high",
		Caveats:          []string{"caveat one"},
		MissingInfo:      "missing",
		SuggestedActions: []string{"action one"},
	}
	printAnswer(answer)
}
