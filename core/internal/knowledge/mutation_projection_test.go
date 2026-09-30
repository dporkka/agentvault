package knowledge

import (
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestMutationAbortPreservesOriginalApprovalAuditAcrossReplay(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	proposal, err := store.CreateMutationProposal(contract.MutationProposal{
		ID:            "mut_approval_audit",
		Kind:          contract.MutationReplace,
		Path:          "note.md",
		Reason:        "verify approval audit",
		Status:        contract.MutationProposed,
		BeforeExists:  true,
		AfterExists:   true,
		BeforeHash:    "before-hash",
		AfterHash:     "after-hash",
		BeforeContent: "before\n",
		AfterContent:  "after\n",
		Diff:          "--- a/note.md\n+++ b/note.md\n",
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}
	approved, err := store.ApproveMutation(proposal.ID, "original-reviewer")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	originalApprovedAt := approved.ApprovedAt
	if originalApprovedAt == "" {
		t.Fatal("approval timestamp missing")
	}
	if _, err := store.StartMutationCommit(proposal.ID); err != nil {
		t.Fatalf("start commit: %v", err)
	}
	aborted, err := store.AbortMutationCommit(proposal.ID, "target remained unchanged")
	if err != nil {
		t.Fatalf("abort commit: %v", err)
	}
	if aborted.Status != contract.MutationApproved {
		t.Fatalf("status=%s want approved", aborted.Status)
	}
	if aborted.ApprovedBy != "original-reviewer" || aborted.ApprovedAt != originalApprovedAt {
		t.Fatalf("abort changed approval audit: %+v", aborted)
	}

	if _, err := database.Exec("DELETE FROM mutation_proposals WHERE id = ?", proposal.ID); err != nil {
		t.Fatalf("delete projection: %v", err)
	}
	if err := store.ReplayJournal(); err != nil {
		t.Fatalf("replay: %v", err)
	}
	replayed, err := store.GetMutationProposal(proposal.ID)
	if err != nil {
		t.Fatalf("get replayed: %v", err)
	}
	if replayed.Status != contract.MutationApproved {
		t.Fatalf("replayed status=%s want approved", replayed.Status)
	}
	if replayed.ApprovedBy != "original-reviewer" || replayed.ApprovedAt != originalApprovedAt {
		t.Fatalf("replay changed approval audit: %+v", replayed)
	}
	if replayed.LastError != "target remained unchanged" {
		t.Fatalf("lastError=%q", replayed.LastError)
	}
}
