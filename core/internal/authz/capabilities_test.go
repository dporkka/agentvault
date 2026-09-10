package authz

import (
	"errors"
	"testing"
	"time"
)

func TestRegistryMintAuthenticateAuthorizeAndRevoke(t *testing.T) {
	registry, err := NewRegistry(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(MintRequest{
		AgentID:      "planner-1",
		Capabilities: []Capability{MutationRead, MutationPropose},
		Scope: Scope{
			PathPrefixes: []string{"30-projects/alpha"},
			Projects:     []string{"alpha"},
			Sessions:     []string{"session-1"},
		},
		ExpiresAt: time.Now().Add(time.Hour).UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
	if issued.Token == "" || issued.Principal.ID == "" {
		t.Fatalf("expected issued token and principal id: %#v", issued)
	}
	principal, err := registry.Authenticate(issued.Token)
	if err != nil {
		t.Fatal(err)
	}
	if principal.AgentID != "planner-1" {
		t.Fatalf("unexpected agent id %q", principal.AgentID)
	}
	allowed := Resource{Path: "30-projects/alpha/plan.md", Project: "alpha", SessionID: "session-1"}
	if err := Authorize(principal, MutationPropose, allowed); err != nil {
		t.Fatalf("expected allowed scope: %v", err)
	}
	if err := Authorize(principal, MutationCommit, allowed); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected missing capability denial, got %v", err)
	}
	if err := Authorize(principal, MutationPropose, Resource{Path: "30-projects/beta/plan.md", Project: "alpha", SessionID: "session-1"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected path denial, got %v", err)
	}
	if err := Authorize(principal, MutationPropose, Resource{Path: "30-projects/alpha/plan.md", Project: "beta", SessionID: "session-1"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected project denial, got %v", err)
	}
	if err := Authorize(principal, MutationPropose, Resource{Path: "30-projects/alpha/plan.md", Project: "alpha", SessionID: "session-2"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected session denial, got %v", err)
	}
	if _, err := registry.Revoke(principal.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := registry.Authenticate(issued.Token); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("expected revoked token rejection, got %v", err)
	}
}

func TestRegistryPersistsOnlyTokenHash(t *testing.T) {
	vault := t.TempDir()
	registry, err := NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	issued, err := registry.Mint(MintRequest{AgentID: "agent", Capabilities: []Capability{MutationRead}})
	if err != nil {
		t.Fatal(err)
	}
	reloaded, err := NewRegistry(vault)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := reloaded.Authenticate(issued.Token); err != nil {
		t.Fatalf("reloaded registry should authenticate token: %v", err)
	}
}

func TestNormalizePathPrefixIsSegmentAware(t *testing.T) {
	principal := Principal{Capabilities: []Capability{MutationRead}, Scope: Scope{PathPrefixes: []string{"foo/bar"}}}
	if err := Authorize(principal, MutationRead, Resource{Path: "foo/bar/note.md"}); err != nil {
		t.Fatal(err)
	}
	if err := Authorize(principal, MutationRead, Resource{Path: "foo/barista/note.md"}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected segment-aware prefix rejection, got %v", err)
	}
}
