package mutations

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestCompetingAgentVaultCommitsDoNotOverwriteEachOther(t *testing.T) {
	vaultPath, store, firstEngine := setupMutationEngine(t)
	secondEngine := New(vaultPath, store, nil)
	path := filepath.Join(vaultPath, "shared.md")
	if err := os.WriteFile(path, []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	firstContent := "first\n"
	secondContent := "second\n"
	first, err := firstEngine.Propose(contract.CreateMutationProposalRequest{
		Kind: contract.MutationReplace, Path: "shared.md", Content: &firstContent, Reason: "first proposal",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := secondEngine.Propose(contract.CreateMutationProposalRequest{
		Kind: contract.MutationReplace, Path: "shared.md", Content: &secondContent, Reason: "second proposal",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := firstEngine.Approve(first.ID, "reviewer"); err != nil {
		t.Fatal(err)
	}
	if _, err := secondEngine.Approve(second.ID, "reviewer"); err != nil {
		t.Fatal(err)
	}

	start := make(chan struct{})
	type outcome struct {
		id  string
		err error
	}
	outcomes := make(chan outcome, 2)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, err := firstEngine.Commit(first.ID)
		outcomes <- outcome{id: first.ID, err: err}
	}()
	go func() {
		defer wg.Done()
		<-start
		_, err := secondEngine.Commit(second.ID)
		outcomes <- outcome{id: second.ID, err: err}
	}()
	close(start)
	wg.Wait()
	close(outcomes)

	successes := 0
	conflicts := 0
	for result := range outcomes {
		if result.err == nil {
			successes++
			continue
		}
		if strings.Contains(result.err.Error(), "conflict") {
			conflicts++
			continue
		}
		t.Fatalf("unexpected commit error for %s: %v", result.id, result.err)
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("successes=%d conflicts=%d, want exactly one of each", successes, conflicts)
	}

	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(current) != firstContent && string(current) != secondContent {
		t.Fatalf("unexpected final content %q", current)
	}

	firstState, err := store.GetMutationProposal(first.ID)
	if err != nil {
		t.Fatal(err)
	}
	secondState, err := store.GetMutationProposal(second.ID)
	if err != nil {
		t.Fatal(err)
	}
	statuses := map[contract.MutationStatus]int{
		firstState.Status:  1,
		secondState.Status: 1,
	}
	// The losing proposal remains approved so it can be rejected or replaced by
	// a new proposal from current state; it is not falsely marked committed.
	if firstState.Status == secondState.Status ||
		!((firstState.Status == contract.MutationCommitted && secondState.Status == contract.MutationApproved) ||
			(firstState.Status == contract.MutationApproved && secondState.Status == contract.MutationCommitted)) {
		t.Fatalf("unexpected proposal states: first=%s second=%s map=%v", firstState.Status, secondState.Status, statuses)
	}
}
