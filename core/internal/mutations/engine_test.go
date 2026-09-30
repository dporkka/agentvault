package mutations

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/db"
	"github.com/agentvault/core/internal/knowledge"
)

func setupMutationEngine(t *testing.T) (string, *knowledge.Store, *Engine) {
	t.Helper()
	vaultPath := t.TempDir()
	if err := os.MkdirAll(filepath.Join(vaultPath, ".agentvault"), 0o755); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(vaultPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	store := knowledge.New(database, vaultPath)
	if err := store.ReplayJournal(); err != nil {
		t.Fatalf("replay: %v", err)
	}
	return vaultPath, store, New(vaultPath, store, nil)
}

func ptr(value string) *string { return &value }

func TestReplaceProposalCommitAndUndo(t *testing.T) {
	vaultPath, store, engine := setupMutationEngine(t)
	path := filepath.Join(vaultPath, "20-projects", "agentvault.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	before := "# AgentVault\n\nold state\n"
	after := "# AgentVault\n\nnew state\n"
	if err := os.WriteFile(path, []byte(before), 0o640); err != nil {
		t.Fatal(err)
	}

	proposal, err := engine.Propose(contract.CreateMutationProposalRequest{
		Kind:    contract.MutationReplace,
		Path:    "20-projects/agentvault.md",
		Content: ptr(after),
		Reason:  "Update architecture decision",
		AgentID: "architect",
	})
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	if proposal.Status != contract.MutationProposed || proposal.BeforeHash == proposal.AfterHash {
		t.Fatalf("unexpected proposal: %+v", proposal)
	}
	if !strings.Contains(proposal.Diff, "-old state") || !strings.Contains(proposal.Diff, "+new state") {
		t.Fatalf("diff missing expected change: %s", proposal.Diff)
	}
	unchanged, _ := os.ReadFile(path)
	if string(unchanged) != before {
		t.Fatalf("proposal mutated file: %q", unchanged)
	}

	approved, err := engine.Approve(proposal.ID, "david")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if approved.Status != contract.MutationApproved || approved.ApprovedBy != "david" {
		t.Fatalf("unexpected approval: %+v", approved)
	}

	committed, err := engine.Commit(proposal.ID)
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if committed.Proposal.Status != contract.MutationCommitted {
		t.Fatalf("unexpected committed proposal: %+v", committed.Proposal)
	}
	current, _ := os.ReadFile(path)
	if string(current) != after {
		t.Fatalf("commit content=%q want=%q", current, after)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("replacement changed file mode to %v", info.Mode().Perm())
	}

	undone, err := engine.Undo(proposal.ID)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if undone.Proposal.Status != contract.MutationUndone {
		t.Fatalf("unexpected undone proposal: %+v", undone.Proposal)
	}
	restored, _ := os.ReadFile(path)
	if string(restored) != before {
		t.Fatalf("undo content=%q want=%q", restored, before)
	}

	loaded, err := store.GetMutationProposal(proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.CommittedAt == "" || loaded.UndoneAt == "" {
		t.Fatalf("lifecycle timestamps missing: %+v", loaded)
	}
}

func TestCreateAndDeleteAreReversible(t *testing.T) {
	t.Run("create", func(t *testing.T) {
		vaultPath, _, engine := setupMutationEngine(t)
		content := "new file\n"
		proposal, err := engine.Propose(contract.CreateMutationProposalRequest{
			Kind: contract.MutationCreate, Path: "10-notes/new.md", Content: ptr(content), Reason: "Create note",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(vaultPath, "10-notes", "new.md")); !os.IsNotExist(err) {
			t.Fatalf("proposal unexpectedly created target: %v", err)
		}
		if _, err := engine.Approve(proposal.ID, "reviewer"); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Commit(proposal.ID); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(filepath.Join(vaultPath, "10-notes", "new.md")); err != nil || string(got) != content {
			t.Fatalf("created content=%q err=%v", got, err)
		}
		if _, err := engine.Undo(proposal.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(vaultPath, "10-notes", "new.md")); !os.IsNotExist(err) {
			t.Fatalf("undo create should remove target: %v", err)
		}
	})

	t.Run("delete", func(t *testing.T) {
		vaultPath, _, engine := setupMutationEngine(t)
		path := filepath.Join(vaultPath, "10-notes", "delete.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		content := "preserve me\n"
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		proposal, err := engine.Propose(contract.CreateMutationProposalRequest{
			Kind: contract.MutationDelete, Path: "10-notes/delete.md", Reason: "Remove stale note",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Approve(proposal.ID, "reviewer"); err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Commit(proposal.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("delete commit left target: %v", err)
		}
		if _, err := engine.Undo(proposal.ID); err != nil {
			t.Fatal(err)
		}
		if got, err := os.ReadFile(path); err != nil || string(got) != content {
			t.Fatalf("restored content=%q err=%v", got, err)
		}
	})
}

func TestCommitAndUndoPreserveThirdPartyEdits(t *testing.T) {
	vaultPath, store, engine := setupMutationEngine(t)
	path := filepath.Join(vaultPath, "note.md")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := "after\n"
	proposal, err := engine.Propose(contract.CreateMutationProposalRequest{
		Kind: contract.MutationReplace, Path: "note.md", Content: ptr(after), Reason: "Change note",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Approve(proposal.ID, "reviewer"); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, []byte("external edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Commit(proposal.ID); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected commit conflict, got %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "external edit\n" {
		t.Fatalf("commit clobbered external edit: %q", got)
	}
	loaded, _ := store.GetMutationProposal(proposal.ID)
	if loaded.Status != contract.MutationApproved {
		t.Fatalf("pre-write conflict should remain approved, got %s", loaded.Status)
	}

	fresh, err := engine.Propose(contract.CreateMutationProposalRequest{
		Kind: contract.MutationReplace, Path: "note.md", Content: ptr(after), Reason: "Change current note",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Approve(fresh.ID, "reviewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Commit(fresh.ID); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("later edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := engine.Undo(fresh.ID); err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("expected undo conflict, got %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "later edit\n" {
		t.Fatalf("undo clobbered later edit: %q", got)
	}
}

func TestRecoveryFinalizesOrConflictsWithoutOverwriting(t *testing.T) {
	t.Run("commit reached after state", func(t *testing.T) {
		vaultPath, store, engine := setupMutationEngine(t)
		path := filepath.Join(vaultPath, "recover.md")
		if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		after := "after\n"
		proposal, err := engine.Propose(contract.CreateMutationProposalRequest{
			Kind: contract.MutationReplace, Path: "recover.md", Content: ptr(after), Reason: "Recover commit",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Approve(proposal.ID, "reviewer"); err != nil {
			t.Fatal(err)
		}
		if _, err := store.StartMutationCommit(proposal.ID); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(after), 0o644); err != nil {
			t.Fatal(err)
		}
		if problems := engine.Recover(); len(problems) != 0 {
			t.Fatalf("recover: %v", problems)
		}
		loaded, _ := store.GetMutationProposal(proposal.ID)
		if loaded.Status != contract.MutationCommitted {
			t.Fatalf("recovery status=%s want committed", loaded.Status)
		}
	})

	t.Run("ambiguous state", func(t *testing.T) {
		vaultPath, store, engine := setupMutationEngine(t)
		path := filepath.Join(vaultPath, "recover.md")
		if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		after := "after\n"
		proposal, err := engine.Propose(contract.CreateMutationProposalRequest{
			Kind: contract.MutationReplace, Path: "recover.md", Content: ptr(after), Reason: "Recover conflict",
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := engine.Approve(proposal.ID, "reviewer"); err != nil {
			t.Fatal(err)
		}
		if _, err := store.StartMutationCommit(proposal.ID); err != nil {
			t.Fatal(err)
		}
		thirdParty := "third party\n"
		if err := os.WriteFile(path, []byte(thirdParty), 0o644); err != nil {
			t.Fatal(err)
		}
		if problems := engine.Recover(); len(problems) != 0 {
			t.Fatalf("recover: %v", problems)
		}
		loaded, _ := store.GetMutationProposal(proposal.ID)
		if loaded.Status != contract.MutationConflicted {
			t.Fatalf("recovery status=%s want conflicted", loaded.Status)
		}
		if got, _ := os.ReadFile(path); string(got) != thirdParty {
			t.Fatalf("recovery overwrote third-party state: %q", got)
		}
	})
}

func TestMutationPathProtections(t *testing.T) {
	vaultPath, _, engine := setupMutationEngine(t)
	content := "x"
	for _, path := range []string{"../escape.md", ".agentvault/config.json", ".git/config", "80-agent-runs/knowledge.journal.jsonl"} {
		t.Run(strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			_, err := engine.Propose(contract.CreateMutationProposalRequest{
				Kind: contract.MutationCreate, Path: path, Content: ptr(content), Reason: "should fail",
			})
			if err == nil {
				t.Fatalf("expected protected path %q to fail", path)
			}
		})
	}

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(vaultPath, "linked")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	_, err := engine.Propose(contract.CreateMutationProposalRequest{
		Kind: contract.MutationCreate, Path: "linked/escape.md", Content: ptr(content), Reason: "escape through symlink",
	})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "symlink") {
		t.Fatalf("expected symlink traversal rejection, got %v", err)
	}
}
