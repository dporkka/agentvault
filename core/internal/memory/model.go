// Package memory defines AgentVault's semantic memory metadata and scoped
// retrieval rules. Markdown files remain canonical; the database stores a
// rebuildable projection of these fields for efficient retrieval.
package memory

import (
	"fmt"
	"strings"
	"time"
)

// Class describes the lifecycle/cognitive role of a memory. It is independent
// of Kind: a semantic memory may be a fact or decision, while an episodic
// memory may be an observation or episode.
type Class string

const (
	ClassWorking    Class = "working"
	ClassEpisodic   Class = "episodic"
	ClassSemantic   Class = "semantic"
	ClassProcedural Class = "procedural"
)

// Valid reports whether c is one of the supported memory classes.
func (c Class) Valid() bool {
	switch c {
	case ClassWorking, ClassEpisodic, ClassSemantic, ClassProcedural:
		return true
	default:
		return false
	}
}

// Kind classifies the semantic meaning of a memory independently of its
// lifecycle class and the note's document type. A semantic memory, for
// example, may be a fact, preference, decision, procedure, or constraint.
type Kind string

const (
	KindObservation Kind = "observation"
	KindEpisode     Kind = "episode"
	KindFact        Kind = "fact"
	KindPreference  Kind = "preference"
	KindDecision    Kind = "decision"
	KindProcedure   Kind = "procedure"
	KindConstraint  Kind = "constraint"
	KindSummary     Kind = "summary"
)

// Valid reports whether k is one of the supported semantic memory kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindObservation, KindEpisode, KindFact, KindPreference, KindDecision,
		KindProcedure, KindConstraint, KindSummary:
		return true
	default:
		return false
	}
}

// Scope describes where a memory is visible. Empty dimensions are broader
// than populated dimensions: an unscoped memory is global, a workspace memory
// is visible within that workspace, and agent/session values narrow it further.
type Scope struct {
	WorkspaceID string `json:"workspaceId,omitempty"`
	AgentID     string `json:"agentId,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
}

// Specificity returns the number of populated scope dimensions.
func (s Scope) Specificity() int {
	n := 0
	if s.WorkspaceID != "" {
		n++
	}
	if s.AgentID != "" {
		n++
	}
	if s.SessionID != "" {
		n++
	}
	return n
}

// VisibleFrom reports whether a memory with this scope is visible from ctx.
// Each populated memory dimension must exactly match the corresponding current
// context. This makes global memories visible everywhere without allowing a
// workspace-, agent-, or session-specific memory to leak into another scope.
func (s Scope) VisibleFrom(ctx Scope) bool {
	return (s.WorkspaceID == "" || s.WorkspaceID == ctx.WorkspaceID) &&
		(s.AgentID == "" || s.AgentID == ctx.AgentID) &&
		(s.SessionID == "" || s.SessionID == ctx.SessionID)
}

// Provenance records where a memory came from. SourceRef should be a stable
// local identifier or URL when one exists; Actor and Model are optional.
type Provenance struct {
	SourceType string `json:"sourceType,omitempty"`
	SourceRef  string `json:"sourceRef,omitempty"`
	Actor      string `json:"actor,omitempty"`
	Model      string `json:"model,omitempty"`
	CapturedAt string `json:"capturedAt,omitempty"`
}

// Metadata is the memory projection attached to a note. Class and Kind are
// orthogonal. For backward-compatible Markdown, a non-empty Kind with an empty
// Class normalizes to ClassSemantic.
type Metadata struct {
	NoteID             string     `json:"noteId"`
	Scope              Scope      `json:"scope"`
	Class              Class      `json:"class,omitempty"`
	Kind               Kind       `json:"kind,omitempty"`
	Confidence         *float64   `json:"confidence,omitempty"`
	Provenance         Provenance `json:"provenance,omitempty"`
	ObservedAt         string     `json:"observedAt,omitempty"`
	ValidFrom          string     `json:"validFrom,omitempty"`
	ValidTo            string     `json:"validTo,omitempty"`
	Supersedes         []string   `json:"supersedes,omitempty"`
	SupersessionReason string     `json:"supersessionReason,omitempty"`
}

// Record combines memory metadata with the minimal note fields useful to a
// retriever. Superseded is derived from the relation table rather than stored
// as mutable state on the note.
type Record struct {
	Metadata
	Title      string `json:"title"`
	Path       string `json:"path"`
	Type       string `json:"type"`
	Project    string `json:"project,omitempty"`
	Status     string `json:"status,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty"`
	Superseded bool   `json:"superseded"`
}

// Query controls memory retrieval.
type Query struct {
	Context           Scope
	Classes           []Class
	Kinds             []Kind
	MinConfidence     *float64
	At                *time.Time
	IncludeSuperseded bool
	Limit             int
}

// Validate rejects metadata that would make retrieval ambiguous or invalid.
func (m Metadata) Validate() error {
	if strings.TrimSpace(m.NoteID) == "" {
		return fmt.Errorf("note id is required")
	}
	if m.Class != "" && !m.Class.Valid() {
		return fmt.Errorf("unsupported memory class %q", m.Class)
	}
	if m.Kind != "" && !m.Kind.Valid() {
		return fmt.Errorf("unsupported memory kind %q", m.Kind)
	}
	if m.Class != "" && m.Kind == "" {
		return fmt.Errorf("memory kind is required when memory class is set")
	}
	if m.Confidence != nil && (*m.Confidence < 0 || *m.Confidence > 1) {
		return fmt.Errorf("confidence must be between 0 and 1")
	}

	validFrom, err := parseOptionalTime("valid_from", m.ValidFrom)
	if err != nil {
		return err
	}
	validTo, err := parseOptionalTime("valid_to", m.ValidTo)
	if err != nil {
		return err
	}
	if _, err := parseOptionalTime("observed_at", m.ObservedAt); err != nil {
		return err
	}
	if _, err := parseOptionalTime("provenance.captured_at", m.Provenance.CapturedAt); err != nil {
		return err
	}
	if !validFrom.IsZero() && !validTo.IsZero() && !validFrom.Before(validTo) {
		return fmt.Errorf("valid_from must be before valid_to")
	}

	seen := make(map[string]struct{}, len(m.Supersedes))
	for _, id := range m.Supersedes {
		id = strings.TrimSpace(id)
		if id == "" {
			return fmt.Errorf("supersedes contains an empty note id")
		}
		if id == m.NoteID {
			return fmt.Errorf("a memory cannot supersede itself")
		}
		if _, ok := seen[id]; ok {
			return fmt.Errorf("supersedes contains duplicate note id %q", id)
		}
		seen[id] = struct{}{}
	}
	return nil
}

// Normalize validates metadata and returns the canonical SQLite projection.
// Existing Markdown that declares memory_kind but predates memory_class is
// treated as semantic memory. RFC3339 timestamps are converted to UTC so
// lexical TEXT ordering in SQLite matches chronological ordering.
func (m Metadata) Normalize() (Metadata, error) {
	if m.Kind != "" && m.Class == "" {
		m.Class = ClassSemantic
	}
	if err := m.Validate(); err != nil {
		return Metadata{}, err
	}

	m.NoteID = strings.TrimSpace(m.NoteID)
	m.Scope.WorkspaceID = strings.TrimSpace(m.Scope.WorkspaceID)
	m.Scope.AgentID = strings.TrimSpace(m.Scope.AgentID)
	m.Scope.SessionID = strings.TrimSpace(m.Scope.SessionID)
	m.ObservedAt = normalizeOptionalTime(m.ObservedAt)
	m.ValidFrom = normalizeOptionalTime(m.ValidFrom)
	m.ValidTo = normalizeOptionalTime(m.ValidTo)
	m.Provenance.CapturedAt = normalizeOptionalTime(m.Provenance.CapturedAt)
	for i := range m.Supersedes {
		m.Supersedes[i] = strings.TrimSpace(m.Supersedes[i])
	}
	return m, nil
}

// IsActive reports whether the memory is temporally valid at the supplied
// instant. ValidFrom is inclusive and ValidTo is exclusive.
func (m Metadata) IsActive(at time.Time) bool {
	from, err := parseOptionalTime("valid_from", m.ValidFrom)
	if err != nil {
		return false
	}
	to, err := parseOptionalTime("valid_to", m.ValidTo)
	if err != nil {
		return false
	}
	if !from.IsZero() && at.Before(from) {
		return false
	}
	if !to.IsZero() && !at.Before(to) {
		return false
	}
	return true
}

func parseOptionalTime(field, value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339: %w", field, err)
	}
	return t, nil
}

func normalizeOptionalTime(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.UTC().Format(time.RFC3339)
}
