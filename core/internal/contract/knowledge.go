package contract

// ProvenanceRecord captures where machine-readable knowledge came from.
// Provenance is append-only: callers should create a new record rather than
// mutating evidence behind an existing fact or memory.
type ProvenanceRecord struct {
	ID         string                 `json:"id"`
	SourceType string                 `json:"sourceType"`
	SourceID   string                 `json:"sourceId,omitempty"`
	AgentID    string                 `json:"agentId,omitempty"`
	SessionID  string                 `json:"sessionId,omitempty"`
	Model      string                 `json:"model,omitempty"`
	Confidence float64                `json:"confidence"`
	ObservedAt string                 `json:"observedAt"`
	Evidence   []ProvenanceEvidence   `json:"evidence,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt  string                 `json:"createdAt"`
}

// CreateProvenanceRequest creates immutable provenance. Confidence defaults to
// 1 when omitted while still allowing an explicit confidence of 0.
type CreateProvenanceRequest struct {
	ID         string                 `json:"id,omitempty"`
	SourceType string                 `json:"sourceType"`
	SourceID   string                 `json:"sourceId,omitempty"`
	AgentID    string                 `json:"agentId,omitempty"`
	SessionID  string                 `json:"sessionId,omitempty"`
	Model      string                 `json:"model,omitempty"`
	Confidence *float64               `json:"confidence,omitempty"`
	ObservedAt string                 `json:"observedAt,omitempty"`
	Evidence   []ProvenanceEvidence   `json:"evidence,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ProvenanceEvidence points at concrete evidence supporting a record.
type ProvenanceEvidence struct {
	Source      string      `json:"source"`
	ID          string      `json:"id,omitempty"`
	Path        string      `json:"path,omitempty"`
	Quote       string      `json:"quote,omitempty"`
	ContentHash string      `json:"contentHash,omitempty"`
	ChunkID     string      `json:"chunkId,omitempty"`
	Span        *SourceSpan `json:"span,omitempty"`
}

// SourceSpan identifies an exact location inside evidence. Line numbers are
// 1-based and inclusive. Byte offsets are 0-based and half-open [start, end).
type SourceSpan struct {
	StartLine int    `json:"startLine,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
	StartByte *int64 `json:"startByte,omitempty"`
	EndByte   *int64 `json:"endByte,omitempty"`
}

// KnowledgeObject is the universal typed envelope shared by notes, projects,
// people, repositories, decisions, tasks, artifacts, agents, and integrations.
// canonicalPath is optional and points at a durable file when one exists.
type KnowledgeObject struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Title         string                 `json:"title"`
	Status        string                 `json:"status,omitempty"`
	Organization  string                 `json:"organization,omitempty"`
	Project       string                 `json:"project,omitempty"`
	CanonicalPath string                 `json:"canonicalPath,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	ProvenanceID  string                 `json:"provenanceId,omitempty"`
	CreatedAt     string                 `json:"createdAt"`
	UpdatedAt     string                 `json:"updatedAt"`
}

// UpsertKnowledgeObjectRequest creates or updates a universal object.
type UpsertKnowledgeObjectRequest struct {
	ID            string                 `json:"id,omitempty"`
	Type          string                 `json:"type"`
	Title         string                 `json:"title"`
	Status        string                 `json:"status,omitempty"`
	Organization  string                 `json:"organization,omitempty"`
	Project       string                 `json:"project,omitempty"`
	CanonicalPath string                 `json:"canonicalPath,omitempty"`
	Data          map[string]interface{} `json:"data,omitempty"`
	ProvenanceID  string                 `json:"provenanceId,omitempty"`
}

// KnowledgeObjectFilter scopes object listing.
type KnowledgeObjectFilter struct {
	Type         string `json:"type,omitempty"`
	Organization string `json:"organization,omitempty"`
	Project      string `json:"project,omitempty"`
	Status       string `json:"status,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}

// ObjectRelation is a typed, optionally temporal edge in the knowledge graph.
type ObjectRelation struct {
	ID           string                 `json:"id"`
	FromObjectID string                 `json:"fromObjectId"`
	ToObjectID   string                 `json:"toObjectId"`
	RelationType string                 `json:"relationType"`
	ValidFrom    string                 `json:"validFrom,omitempty"`
	ValidTo      string                 `json:"validTo,omitempty"`
	Confidence   float64                `json:"confidence"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    string                 `json:"createdAt"`
	UpdatedAt    string                 `json:"updatedAt"`
}

// CreateObjectRelationRequest creates a relationship between two objects.
type CreateObjectRelationRequest struct {
	ID           string                 `json:"id,omitempty"`
	FromObjectID string                 `json:"fromObjectId"`
	ToObjectID   string                 `json:"toObjectId"`
	RelationType string                 `json:"relationType"`
	ValidFrom    string                 `json:"validFrom,omitempty"`
	ValidTo      string                 `json:"validTo,omitempty"`
	Confidence   *float64               `json:"confidence,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryRecord stores one durable machine-authored memory. Class describes its
// lifecycle/cognitive role; Kind describes its semantic meaning. They use the
// same vocabulary as file-backed Markdown memories.
type MemoryRecord struct {
	ID           string                 `json:"id"`
	MemoryClass  string                 `json:"memoryClass"`
	MemoryKind   string                 `json:"memoryKind,omitempty"`
	ScopeType    string                 `json:"scopeType"`
	ScopeID      string                 `json:"scopeId"`
	Content      string                 `json:"content"`
	ObjectID     string                 `json:"objectId,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	Confidence   float64                `json:"confidence"`
	ValidFrom    string                 `json:"validFrom,omitempty"`
	ValidTo      string                 `json:"validTo,omitempty"`
	SupersedesID string                 `json:"supersedesId,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    string                 `json:"createdAt"`
	UpdatedAt    string                 `json:"updatedAt"`
}

// CreateMemoryRequest records a memory. memoryClass must be working, episodic,
// semantic, or procedural. memoryKind is optional but, when supplied, must use
// the shared semantic-kind vocabulary.
type CreateMemoryRequest struct {
	ID           string                 `json:"id,omitempty"`
	MemoryClass  string                 `json:"memoryClass"`
	MemoryKind   string                 `json:"memoryKind,omitempty"`
	ScopeType    string                 `json:"scopeType"`
	ScopeID      string                 `json:"scopeId"`
	Content      string                 `json:"content"`
	ObjectID     string                 `json:"objectId,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	Confidence   *float64               `json:"confidence,omitempty"`
	ValidFrom    string                 `json:"validFrom,omitempty"`
	ValidTo      string                 `json:"validTo,omitempty"`
	SupersedesID string                 `json:"supersedesId,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// MemoryCandidateStatus is the explicit review lifecycle for proposed
// semantic memory. Candidates are not durable MemoryRecords until a terminal
// review action materializes one.
type MemoryCandidateStatus string

const (
	MemoryCandidatePending    MemoryCandidateStatus = "pending"
	MemoryCandidateAccepted   MemoryCandidateStatus = "accepted"
	MemoryCandidateRejected   MemoryCandidateStatus = "rejected"
	MemoryCandidateMerged     MemoryCandidateStatus = "merged"
	MemoryCandidateSuperseded MemoryCandidateStatus = "superseded"
)

// MemoryCandidate is a reviewable semantic-memory proposal derived from one
// provenance-backed episode. Scope and provenance are inherited from the source
// episode so an extractor cannot silently widen visibility or replace evidence.
type MemoryCandidate struct {
	ID              string                 `json:"id"`
	SourceEpisodeID string                 `json:"sourceEpisodeId"`
	MemoryKind      string                 `json:"memoryKind"`
	ScopeType       string                 `json:"scopeType"`
	ScopeID         string                 `json:"scopeId"`
	Content         string                 `json:"content"`
	ObjectID        string                 `json:"objectId,omitempty"`
	ProvenanceID    string                 `json:"provenanceId"`
	Confidence      float64                `json:"confidence"`
	Status          MemoryCandidateStatus  `json:"status"`
	ProposedBy      string                 `json:"proposedBy,omitempty"`
	ReviewedBy      string                 `json:"reviewedBy,omitempty"`
	ReviewReason    string                 `json:"reviewReason,omitempty"`
	ResultMemoryID  string                 `json:"resultMemoryId,omitempty"`
	TargetMemoryID  string                 `json:"targetMemoryId,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt       string                 `json:"createdAt"`
	UpdatedAt       string                 `json:"updatedAt"`
	ReviewedAt      string                 `json:"reviewedAt,omitempty"`
}

// CreateMemoryCandidateRequest proposes semantic memory from one
// provenance-backed episode. The candidate inherits scope/provenance from that
// episode and never materializes durable memory by itself.
type CreateMemoryCandidateRequest struct {
	ID         string                 `json:"id,omitempty"`
	EpisodeID  string                 `json:"episodeId"`
	MemoryKind string                 `json:"memoryKind"`
	Content    string                 `json:"content"`
	ObjectID   string                 `json:"objectId,omitempty"`
	Confidence *float64               `json:"confidence,omitempty"`
	ProposedBy string                 `json:"proposedBy,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// ExtractMemoryCandidatesRequest runs deterministic semantic extraction for one
// existing provenance-backed episode. It never performs terminal review.
type ExtractMemoryCandidatesRequest struct {
	EpisodeID string `json:"episodeId"`
}

// MemoryCandidateFilter scopes candidate review queues.
type MemoryCandidateFilter struct {
	Status     MemoryCandidateStatus `json:"status,omitempty"`
	ScopeType  string                `json:"scopeType,omitempty"`
	ScopeID    string                `json:"scopeId,omitempty"`
	MemoryKind string                `json:"memoryKind,omitempty"`
	Limit      int                   `json:"limit,omitempty"`
}

// ReviewMemoryCandidateRequest accepts or rejects a candidate.
type ReviewMemoryCandidateRequest struct {
	ReviewedBy string `json:"reviewedBy"`
	Reason     string `json:"reason,omitempty"`
}

// SupersedeMemoryCandidateRequest materializes the candidate as a replacement
// for one existing memory in the same scope/kind.
type SupersedeMemoryCandidateRequest struct {
	ReviewedBy     string `json:"reviewedBy"`
	TargetMemoryID string `json:"targetMemoryId"`
	Reason         string `json:"reason,omitempty"`
}

// MergeMemoryCandidateRequest lets the reviewer supply the explicit merged
// content that will replace an existing memory in the same scope/kind.
type MergeMemoryCandidateRequest struct {
	ReviewedBy     string `json:"reviewedBy"`
	TargetMemoryID string `json:"targetMemoryId"`
	MergedContent  string `json:"mergedContent"`
	Reason         string `json:"reason,omitempty"`
}

// AgentSession is a durable workspace for one agent objective.
type AgentSession struct {
	ID        string                 `json:"id"`
	AgentID   string                 `json:"agentId"`
	Project   string                 `json:"project,omitempty"`
	Objective string                 `json:"objective"`
	Status    string                 `json:"status"`
	Branch    string                 `json:"branch,omitempty"`
	Worktree  string                 `json:"worktree,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
	StartedAt string                 `json:"startedAt"`
	UpdatedAt string                 `json:"updatedAt"`
	EndedAt   string                 `json:"endedAt,omitempty"`
	Events    []SessionEvent         `json:"events,omitempty"`
}

// StartAgentSessionRequest creates a durable agent session.
type StartAgentSessionRequest struct {
	ID        string                 `json:"id,omitempty"`
	AgentID   string                 `json:"agentId"`
	Project   string                 `json:"project,omitempty"`
	Objective string                 `json:"objective"`
	Branch    string                 `json:"branch,omitempty"`
	Worktree  string                 `json:"worktree,omitempty"`
	Context   map[string]interface{} `json:"context,omitempty"`
}

// SessionEvent records a durable event in an agent session.
type SessionEvent struct {
	ID           string                 `json:"id"`
	SessionID    string                 `json:"sessionId"`
	EventType    string                 `json:"eventType"`
	Payload      map[string]interface{} `json:"payload,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	CreatedAt    string                 `json:"createdAt"`
}

// AppendSessionEventRequest adds an event to a session.
type AppendSessionEventRequest struct {
	ID           string                 `json:"id,omitempty"`
	EventType    string                 `json:"eventType"`
	Payload      map[string]interface{} `json:"payload,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
}

// CloseAgentSessionRequest marks a session terminal while preserving its history.
type CloseAgentSessionRequest struct {
	Status string `json:"status,omitempty"`
}

// EpisodeRecord is an immutable occurrence in durable agent knowledge.
// occurredAt describes when the event happened; provenance.observedAt
// independently describes when AgentVault learned about it.
type EpisodeRecord struct {
	ID           string                 `json:"id"`
	ScopeType    string                 `json:"scopeType"`
	ScopeID      string                 `json:"scopeId"`
	EventType    string                 `json:"eventType"`
	Summary      string                 `json:"summary"`
	ObjectIDs    []string               `json:"objectIds,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	OccurredAt   string                 `json:"occurredAt"`
	EndedAt      string                 `json:"endedAt,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    string                 `json:"createdAt"`
}

// CreateEpisodeRequest records a new immutable occurrence.
type CreateEpisodeRequest struct {
	ID           string                 `json:"id,omitempty"`
	ScopeType    string                 `json:"scopeType"`
	ScopeID      string                 `json:"scopeId"`
	EventType    string                 `json:"eventType"`
	Summary      string                 `json:"summary"`
	ObjectIDs    []string               `json:"objectIds,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	OccurredAt   string                 `json:"occurredAt,omitempty"`
	EndedAt      string                 `json:"endedAt,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// TemporalFact is a provenance-backed truth claim with independent validity
// and observation clocks. Supersession never deletes history: the new fact
// points at the prior fact and the SQLite projection derives reverse links.
type TemporalFact struct {
	ID           string                 `json:"id"`
	SubjectID    string                 `json:"subjectId"`
	Predicate    string                 `json:"predicate"`
	ObjectID     string                 `json:"objectId,omitempty"`
	Value        string                 `json:"value,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	Confidence   float64                `json:"confidence"`
	ValidFrom    string                 `json:"validFrom,omitempty"`
	ValidTo      string                 `json:"validTo,omitempty"`
	SupersedesID string                 `json:"supersedesId,omitempty"`
	SupersededAt string                 `json:"supersededAt,omitempty"`
	SupersededBy string                 `json:"supersededBy,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt    string                 `json:"createdAt"`
	UpdatedAt    string                 `json:"updatedAt"`
}

// CreateTemporalFactRequest creates a temporal fact. Exactly one of objectId
// or a non-empty value is sufficient; callers may provide both when a stable
// object identity also needs a human-readable value.
type CreateTemporalFactRequest struct {
	ID           string                 `json:"id,omitempty"`
	SubjectID    string                 `json:"subjectId"`
	Predicate    string                 `json:"predicate"`
	ObjectID     string                 `json:"objectId,omitempty"`
	Value        string                 `json:"value,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	Confidence   *float64               `json:"confidence,omitempty"`
	ValidFrom    string                 `json:"validFrom,omitempty"`
	ValidTo      string                 `json:"validTo,omitempty"`
	SupersedesID string                 `json:"supersedesId,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// EntityProfile is a deterministic standing view over one durable knowledge
// object and the temporal facts currently visible at AsOf. It is derived state:
// objects and facts remain the canonical journal-backed records.
type EntityProfile struct {
	Subject KnowledgeObject `json:"subject"`
	AsOf    string          `json:"asOf"`
	Facts   []TemporalFact  `json:"facts"`
}

// TimelineFilter scopes the derived activity stream across canonical capture,
// memory, episode, durable session-event, and mutation sources.
type TimelineFilter struct {
	Project   string `json:"project,omitempty"`
	AgentID   string `json:"agentId,omitempty"`
	SessionID string `json:"sessionId,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Since     string `json:"since,omitempty"`
	Until     string `json:"until,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// TimelineItem is a normalized projection for human activity browsing. It is
// intentionally derived state; source records remain canonical in Markdown or
// the append-only knowledge journal.
type TimelineItem struct {
	Kind         string                 `json:"kind"`
	ID           string                 `json:"id"`
	Title        string                 `json:"title,omitempty"`
	Summary      string                 `json:"summary,omitempty"`
	Project      string                 `json:"project,omitempty"`
	AgentID      string                 `json:"agentId,omitempty"`
	SessionID    string                 `json:"sessionId,omitempty"`
	ScopeType    string                 `json:"scopeType,omitempty"`
	ScopeID      string                 `json:"scopeId,omitempty"`
	EventType    string                 `json:"eventType,omitempty"`
	ObjectIDs    []string               `json:"objectIds,omitempty"`
	ProvenanceID string                 `json:"provenanceId,omitempty"`
	OccurredAt   string                 `json:"occurredAt"`
	CreatedAt    string                 `json:"createdAt"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}
