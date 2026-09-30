package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/agentvault/core/internal/contract"
)

func TestUpdateNoteRejectsStaleContentHash(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/notes/note_2024_01_15_123")
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	defer resp.Body.Close()

	var note contract.NoteDetail
	if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
		t.Fatalf("decode note: %v", err)
	}
	if note.ContentHash == "" {
		t.Fatal("expected contentHash on note read")
	}

	path := filepath.Join(vaultPath, "10-notes", "test-note.md")
	external := []byte("---\nid: note_2024_01_15_123\ntype: note\ntitle: Externally Edited\nproject: test-project\ntags: [go, api]\ncreated: 2024-01-15T10:00:00Z\nupdated: 2024-01-15T12:30:00Z\n---\n\nExternal edit wins.\n")
	if err := os.WriteFile(path, external, 0644); err != nil {
		t.Fatalf("external write: %v", err)
	}

	reqBody, _ := json.Marshal(contract.UpdateNoteRequest{
		Title:               ptrString("Stale Client"),
		Content:             ptrString("stale content"),
		ExpectedContentHash: &note.ContentHash,
	})
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/notes/note_2024_01_15_123", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 conflict, got %d", resp2.StatusCode)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read after conflict: %v", err)
	}
	if string(after) != string(external) {
		t.Fatal("stale update overwrote an external edit")
	}
}

func TestUpdateNoteWithCurrentHashReturnsNewHash(t *testing.T) {
	vaultPath, database := setupTestVault(t)
	defer database.Close()

	ts := newTestServer(t, vaultPath, database)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/notes/note_2024_01_15_123")
	if err != nil {
		t.Fatalf("get note: %v", err)
	}
	defer resp.Body.Close()

	var note contract.NoteDetail
	if err := json.NewDecoder(resp.Body).Decode(&note); err != nil {
		t.Fatalf("decode note: %v", err)
	}

	reqBody, _ := json.Marshal(contract.UpdateNoteRequest{
		Content:             ptrString("# Test Note\n\nUpdated safely.\n"),
		ExpectedContentHash: &note.ContentHash,
	})
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/notes/note_2024_01_15_123", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("update note: %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var updated contract.UpdateNoteResponse
	if err := json.NewDecoder(resp2.Body).Decode(&updated); err != nil {
		t.Fatalf("decode update response: %v", err)
	}
	if updated.ContentHash == "" || updated.ContentHash == note.ContentHash {
		t.Fatalf("expected a new contentHash, before=%q after=%q", note.ContentHash, updated.ContentHash)
	}

	raw, err := os.ReadFile(filepath.Join(vaultPath, "10-notes", "test-note.md"))
	if err != nil {
		t.Fatalf("read updated note: %v", err)
	}
	sum := sha256.Sum256(raw)
	if updated.ContentHash != hex.EncodeToString(sum[:]) {
		t.Fatalf("response hash does not match written file")
	}
}

func ptrString(v string) *string { return &v }
