package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
	"github.com/agentvault/core/internal/knowledge"
)

func TestMutationHTTPAPIRequiresExplicitApprovalAndSupportsUndo(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	path := filepath.Join(vaultPath, "10-notes", "http-mutation.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	before := "# Before\n"
	after := "# After\n"
	if err := os.WriteFile(path, []byte(before), 0o644); err != nil {
		t.Fatal(err)
	}

	server := NewServer(vaultPath, database)
	server.RegisterRoutes()
	ts := httptest.NewServer(server.mux)
	defer ts.Close()

	call := func(method, route string, body interface{}, target interface{}, want int) {
		t.Helper()
		var payload []byte
		if body != nil {
			var err error
			payload, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, err := http.NewRequest(method, ts.URL+route, bytes.NewReader(payload))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-AgentVault-Token", server.AuthToken())
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != want {
			var failure map[string]interface{}
			_ = json.NewDecoder(resp.Body).Decode(&failure)
			t.Fatalf("%s %s status=%d want=%d body=%v", method, route, resp.StatusCode, want, failure)
		}
		if target != nil {
			if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
				t.Fatal(err)
			}
		}
	}

	var proposed contract.MutationProposal
	call(http.MethodPost, "/mutations", map[string]interface{}{
		"kind": "replace", "path": "10-notes/http-mutation.md", "content": after,
		"reason": "HTTP mutation test", "agentId": "test-agent",
	}, &proposed, http.StatusCreated)
	if proposed.Status != contract.MutationProposed || proposed.Diff == "" {
		t.Fatalf("unexpected proposal: %+v", proposed)
	}
	if got, _ := os.ReadFile(path); string(got) != before {
		t.Fatalf("proposal changed file: %q", got)
	}

	call(http.MethodPost, "/mutations/"+proposed.ID+"/commit", nil, nil, http.StatusConflict)
	if got, _ := os.ReadFile(path); string(got) != before {
		t.Fatalf("unapproved commit changed file: %q", got)
	}

	var approved contract.MutationProposal
	call(http.MethodPost, "/mutations/"+proposed.ID+"/approve", map[string]interface{}{"approvedBy": "david"}, &approved, http.StatusOK)
	if approved.Status != contract.MutationApproved || approved.ApprovedBy != "david" {
		t.Fatalf("unexpected approval: %+v", approved)
	}

	var committed contract.MutationResult
	call(http.MethodPost, "/mutations/"+proposed.ID+"/commit", nil, &committed, http.StatusOK)
	if committed.Proposal.Status != contract.MutationCommitted {
		t.Fatalf("unexpected commit: %+v", committed)
	}
	if got, _ := os.ReadFile(path); string(got) != after {
		t.Fatalf("commit content=%q", got)
	}

	var listed []contract.MutationProposal
	call(http.MethodGet, "/mutations?status=committed&agentId=test-agent", nil, &listed, http.StatusOK)
	if len(listed) != 1 || listed[0].ID != proposed.ID {
		t.Fatalf("unexpected mutation list: %+v", listed)
	}

	var undone contract.MutationResult
	call(http.MethodPost, "/mutations/"+proposed.ID+"/undo", nil, &undone, http.StatusOK)
	if undone.Proposal.Status != contract.MutationUndone {
		t.Fatalf("unexpected undo: %+v", undone)
	}
	if got, _ := os.ReadFile(path); string(got) != before {
		t.Fatalf("undo content=%q", got)
	}
}

func TestHTTPServerRecoversInterruptedMutationOnStartup(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()
	path := filepath.Join(vaultPath, "recover-http.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	store := knowledge.New(database, vaultPath)
	engine := NewServer(vaultPath, database).mutations
	after := "after\n"
	proposal, err := engine.Propose(contract.CreateMutationProposalRequest{
		Kind: contract.MutationReplace, Path: "recover-http.txt", Content: &after, Reason: "startup recovery",
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

	restarted := NewServer(vaultPath, database)
	if restarted.knowledgeInitErr != nil {
		t.Fatalf("knowledge startup failed: %v", restarted.knowledgeInitErr)
	}
	if restarted.mutationInitErr != nil {
		t.Fatalf("mutation startup recovery failed: %v", restarted.mutationInitErr)
	}
	loaded, err := restarted.knowledge.GetMutationProposal(proposal.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != contract.MutationCommitted {
		t.Fatalf("startup recovery status=%s want committed", loaded.Status)
	}
}
