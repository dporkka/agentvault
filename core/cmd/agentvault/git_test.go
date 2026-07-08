package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunGitInit(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(vp, ".git")); os.IsNotExist(err) {
		t.Error("expected .git directory to be created")
	}
}

func TestRunGitInitAlreadyRepo(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("first runGitInit returned error: %v", err)
	}
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("second runGitInit returned error: %v", err)
	}
}

func TestRunGitStatusClean(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	if err := runGitStatus(cmd, []string{}); err != nil {
		t.Fatalf("runGitStatus returned error: %v", err)
	}
}

func TestRunGitAddAndCommit(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vp, "hello.md"), []byte("hello"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	gitCommitMessage = "initial"
	if err := runGitAdd(cmd, []string{"hello.md"}); err != nil {
		t.Fatalf("runGitAdd returned error: %v", err)
	}
	if err := runGitCommit(cmd, []string{}); err != nil {
		t.Fatalf("runGitCommit returned error: %v", err)
	}
}

func TestRunGitDiff(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vp, "tracked.md"), []byte("original"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGitAdd(cmd, []string{"tracked.md"}); err != nil {
		t.Fatalf("runGitAdd returned error: %v", err)
	}
	gitCommitMessage = "add tracked"
	if err := runGitCommit(cmd, []string{}); err != nil {
		t.Fatalf("runGitCommit returned error: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vp, "tracked.md"), []byte("modified"), 0644); err != nil {
		t.Fatalf("failed to modify file: %v", err)
	}

	if err := runGitDiff(cmd, []string{"tracked.md"}); err != nil {
		t.Fatalf("runGitDiff returned error: %v", err)
	}
}

func TestRunGitLog(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vp, "log.md"), []byte("log"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGitAdd(cmd, []string{"log.md"}); err != nil {
		t.Fatalf("runGitAdd returned error: %v", err)
	}
	gitCommitMessage = "first"
	if err := runGitCommit(cmd, []string{}); err != nil {
		t.Fatalf("runGitCommit returned error: %v", err)
	}

	gitLogLimit = 10
	if err := runGitLog(cmd, []string{}); err != nil {
		t.Fatalf("runGitLog returned error: %v", err)
	}
}

func TestRunGitSnapshot(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	// Snapshot with nothing to commit should report clean working tree.
	gitSnapshotMessage = "snap"
	if err := runGitSnapshot(cmd, []string{}); err != nil {
		t.Fatalf("runGitSnapshot returned error: %v", err)
	}

	// Make a change and snapshot again.
	if err := os.WriteFile(filepath.Join(vp, "snap.md"), []byte("snap"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	if err := runGitSnapshot(cmd, []string{}); err != nil {
		t.Fatalf("runGitSnapshot returned error after change: %v", err)
	}
}

func TestRunGitSnapshotNoMessageUsesDefault(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	if err := os.WriteFile(filepath.Join(vp, "snap-default.md"), []byte("snap"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
	gitSnapshotMessage = ""
	if err := runGitSnapshot(cmd, []string{}); err != nil {
		t.Fatalf("runGitSnapshot returned error: %v", err)
	}
}

func TestRunGitCommitMessageFlagRequired(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	// The command's MarkFlagRequired is enforced by cobra during parsing, not by
	// the RunE function. Verify the flag is registered.
	if gitCommitCmd.Flags().Lookup("message") == nil {
		t.Fatal("expected 'message' flag to be registered on git commit")
	}
}

func TestRunGitDiffNoChanges(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	if err := runGitDiff(cmd, []string{}); err != nil {
		t.Fatalf("runGitDiff returned error: %v", err)
	}
}

func TestRunGitLogNoCommits(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	gitLogLimit = 10
	if err := runGitLog(cmd, []string{}); err != nil {
		t.Fatalf("runGitLog returned error: %v", err)
	}
}

func TestRunGitAddNonexistentFileReturnsError(t *testing.T) {
	vp := setupTestVault(t)
	vaultPath = vp

	cmd := cobraCommandArgs([]string{})
	if err := runGitInit(cmd, []string{}); err != nil {
		t.Fatalf("runGitInit returned error: %v", err)
	}

	if err := runGitAdd(cmd, []string{"nonexistent.md"}); err == nil {
		t.Fatal("expected error adding nonexistent file")
	}
}

func TestGitCommandHelp(t *testing.T) {
	if gitCmd.Short == "" {
		t.Error("expected git command to have a short description")
	}
	if len(gitCmd.Commands()) == 0 {
		t.Error("expected git command to have subcommands")
	}
}
