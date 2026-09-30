package authz

import (
	"errors"
	"runtime"
	"testing"
)

func TestAuthorizeRejectsProtectedAgentVaultPathsCaseInsensitively(t *testing.T) {
	principal := Principal{Capabilities: []Capability{MutationPropose}}
	for _, path := range []string{
		".agentvault/capabilities.json",
		".AgentVault/capabilities.json",
		".GIT/config",
		"80-agent-runs/KNOWLEDGE.JOURNAL.JSONL",
	} {
		if err := Authorize(principal, MutationPropose, Resource{Path: path}); !errors.Is(err, ErrForbidden) {
			t.Fatalf("path %q should be forbidden, got %v", path, err)
		}
	}
}

func TestNormalizeRelativePathRejectsAbsolutePaths(t *testing.T) {
	if _, err := normalizeRelativePath("/tmp/note.md"); err == nil {
		t.Fatal("expected Unix absolute path rejection")
	}
	if runtime.GOOS == "windows" {
		if _, err := normalizeRelativePath(`C:\\vault\\note.md`); err == nil {
			t.Fatal("expected Windows drive path rejection")
		}
	}
}

func TestMintRejectsProtectedPathScope(t *testing.T) {
	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Mint(MintRequest{
		AgentID: "agent", Capabilities: []Capability{MutationPropose},
		Scope: Scope{PathPrefixes: []string{".AgentVault"}},
	}); err == nil {
		t.Fatal("expected protected path scope to be rejected")
	}
}
