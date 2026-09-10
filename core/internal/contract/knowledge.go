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

// ProvenanceEvidence points at concrete evidence supporting a record.
type ProvenanceEvidence struct {
	Source string `json:"source"`
	ID     string `json:"id,omitempty"`
	Path   string `json:"path,omitempty"`
	Quote  string `json:"quote,omitempty"`
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

// MemoryRecord stores one durable memory with explicit class, scope,
// provenance, confidence, and temporal validity.
type MemoryRecord struct {
	ID           string                 `json:"id"`
	MemoryType   string                 `json:"memoryType"`
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

// CreateMemoryRequest records a memory. memoryType must be working, episodic,
// semantic, or procedural.
type CreateMemoryRequest struct {
	ID           string                 `json:"id,omitempty"`
	MemoryType   string                 `json:"memoryType"`
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
