package knowledge

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agentvault/core/internal/contract"
)

func TestMutationReplayRejectsMismatchedTransitionStatus(t *testing.T) {
	store, database, _ := setupStore(t)
	defer database.Close()

	proposal, err := store.CreateMutationProposal(contract.MutationProposal{
		ID:            "mut_tampered_transition",
		Kind:          contract.MutationReplace,
		Path:          "note.md",
		Reason:        "replay validation",
		Status:        contract.MutationProposed,
		BeforeExists:  true,
		AfterExists:   true,
		BeforeHash:    "before",
		AfterHash:     "after",
		BeforeContent: "before\n",
		AfterContent:  "after\n",
		Diff:          "diff",
	})
	if err != nil {
		t.Fatalf("create proposal: %v", err)
	}

	transition := mutationTransition{
		ID:         proposal.ID,
		Status:     contract.MutationCommitted,
		Actor:      "tampered-reviewer",
		OccurredAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	payload, err := json.Marshal(transition)
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.projectMutationJournalEvent(JournalEvent{
		Version:   journalVersion,
		ID:        "evt_tampered",
		Type:      eventMutationApproved,
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:   payload,
	})
	if err == nil {
		t.Fatal("mismatched mutation.approved -> committed transition must be rejected")
	}

	loaded, err := store.GetMutationProposal(proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != contract.MutationProposed || loaded.ApprovedBy != "" {
		t.Fatalf("invalid event changed projection: %+v", loaded)
	}
}
