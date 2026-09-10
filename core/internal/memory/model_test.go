package memory

import (
	"strings"
	"testing"
	"time"
)

func TestScopeVisibleFrom(t *testing.T) {
	ctx := Scope{WorkspaceID: "workspace-a", AgentID: "agent-a", SessionID: "session-a"}

	tests := []struct {
		name  string
		scope Scope
		want  bool
	}{
		{name: "global", scope: Scope{}, want: true},
		{name: "workspace", scope: Scope{WorkspaceID: "workspace-a"}, want: true},
		{name: "workspace mismatch", scope: Scope{WorkspaceID: "workspace-b"}, want: false},
		{name: "agent", scope: Scope{WorkspaceID: "workspace-a", AgentID: "agent-a"}, want: true},
		{name: "agent mismatch", scope: Scope{WorkspaceID: "workspace-a", AgentID: "agent-b"}, want: false},
		{name: "session", scope: Scope{WorkspaceID: "workspace-a", AgentID: "agent-a", SessionID: "session-a"}, want: true},
		{name: "session mismatch", scope: Scope{WorkspaceID: "workspace-a", AgentID: "agent-a", SessionID: "session-b"}, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.scope.VisibleFrom(ctx); got != tt.want {
				t.Fatalf("VisibleFrom() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestScopeSpecificity(t *testing.T) {
	tests := []struct {
		scope Scope
		want  int
	}{
		{Scope{}, 0},
		{Scope{WorkspaceID: "w"}, 1},
		{Scope{WorkspaceID: "w", AgentID: "a"}, 2},
		{Scope{WorkspaceID: "w", AgentID: "a", SessionID: "s"}, 3},
	}
	for _, tt := range tests {
		if got := tt.scope.Specificity(); got != tt.want {
			t.Fatalf("Specificity(%+v) = %d, want %d", tt.scope, got, tt.want)
		}
	}
}

func TestMetadataValidate(t *testing.T) {
	validConfidence := 0.8
	base := Metadata{
		NoteID:     "note-1",
		Kind:       KindFact,
		Confidence: &validConfidence,
		ObservedAt: "2026-09-10T10:00:00Z",
		ValidFrom:  "2026-09-10T10:00:00Z",
		ValidTo:    "2026-09-11T10:00:00Z",
		Provenance: Provenance{CapturedAt: "2026-09-10T09:00:00Z"},
		Supersedes: []string{"note-0"},
	}
	if err := base.Validate(); err != nil {
		t.Fatalf("valid metadata rejected: %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*Metadata)
		contains string
	}{
		{name: "missing note id", mutate: func(m *Metadata) { m.NoteID = "" }, contains: "note id"},
		{name: "bad kind", mutate: func(m *Metadata) { m.Kind = Kind("other") }, contains: "unsupported memory kind"},
		{name: "bad confidence", mutate: func(m *Metadata) { v := 1.1; m.Confidence = &v }, contains: "confidence"},
		{name: "bad observed time", mutate: func(m *Metadata) { m.ObservedAt = "yesterday" }, contains: "observed_at"},
		{name: "inverted validity", mutate: func(m *Metadata) { m.ValidFrom = "2026-09-12T10:00:00Z" }, contains: "valid_from must be before valid_to"},
		{name: "self supersession", mutate: func(m *Metadata) { m.Supersedes = []string{"note-1"} }, contains: "cannot supersede itself"},
		{name: "duplicate supersession", mutate: func(m *Metadata) { m.Supersedes = []string{"note-0", "note-0"} }, contains: "duplicate"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := base
			m.Supersedes = append([]string(nil), base.Supersedes...)
			tt.mutate(&m)
			err := m.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("Validate() error = %v, want substring %q", err, tt.contains)
			}
		})
	}
}

func TestMetadataIsActiveBoundaries(t *testing.T) {
	metadata := Metadata{
		NoteID:    "note-1",
		ValidFrom: "2026-09-10T10:00:00Z",
		ValidTo:   "2026-09-10T12:00:00Z",
	}

	parse := func(value string) time.Time {
		t.Helper()
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatal(err)
		}
		return parsed
	}

	if metadata.IsActive(parse("2026-09-10T09:59:59Z")) {
		t.Fatal("memory should be inactive before valid_from")
	}
	if !metadata.IsActive(parse("2026-09-10T10:00:00Z")) {
		t.Fatal("valid_from should be inclusive")
	}
	if !metadata.IsActive(parse("2026-09-10T11:59:59Z")) {
		t.Fatal("memory should be active before valid_to")
	}
	if metadata.IsActive(parse("2026-09-10T12:00:00Z")) {
		t.Fatal("valid_to should be exclusive")
	}
}
