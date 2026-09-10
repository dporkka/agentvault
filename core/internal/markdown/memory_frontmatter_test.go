package markdown

import "testing"

func TestParseMemoryFrontmatter(t *testing.T) {
	doc, err := ParseBytes([]byte(`---
id: fact-1
type: note
title: Preferred deployment target
workspace_id: adacavo
agent_id: planner
session_id: session-42
memory_kind: preference
confidence: 0.91
provenance:
  source_type: conversation
  source_ref: conversation-12
  actor: user
  model: gpt-test
  captured_at: 2026-09-10T10:00:00Z
observed_at: 2026-09-10T10:01:00Z
valid_from: 2026-09-10T10:01:00Z
valid_to: 2027-09-10T10:01:00Z
supersedes: [fact-0]
supersession_reason: newer explicit preference
custom_field: preserved
---
Use the lower-maintenance deployment target.
`))
	if err != nil {
		t.Fatalf("ParseBytes() error: %v", err)
	}

	fm := doc.Frontmatter
	if fm.WorkspaceID != "adacavo" || fm.AgentID != "planner" || fm.SessionID != "session-42" {
		t.Fatalf("scope fields = %q/%q/%q", fm.WorkspaceID, fm.AgentID, fm.SessionID)
	}
	if fm.MemoryKind != "preference" {
		t.Fatalf("memory kind = %q", fm.MemoryKind)
	}
	if fm.Confidence == nil || *fm.Confidence != 0.91 {
		t.Fatalf("confidence = %v", fm.Confidence)
	}
	if fm.Provenance.SourceType != "conversation" || fm.Provenance.SourceRef != "conversation-12" {
		t.Fatalf("provenance = %+v", fm.Provenance)
	}
	if fm.ObservedAt != "2026-09-10T10:01:00Z" || fm.ValidFrom == "" || fm.ValidTo == "" {
		t.Fatalf("temporal fields = observed:%q from:%q to:%q", fm.ObservedAt, fm.ValidFrom, fm.ValidTo)
	}
	if len(fm.Supersedes) != 1 || fm.Supersedes[0] != "fact-0" {
		t.Fatalf("supersedes = %#v", fm.Supersedes)
	}
	if fm.SupersessionReason != "newer explicit preference" {
		t.Fatalf("supersession reason = %q", fm.SupersessionReason)
	}
	if got := fm.Extra["custom_field"]; got != "preserved" {
		t.Fatalf("custom inline frontmatter field = %#v", got)
	}
}
