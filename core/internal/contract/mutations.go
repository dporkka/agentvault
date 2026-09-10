package contract

// MutationKind is the supported single-file mutation operation. The first
// transactional slice deliberately limits proposals to one file so commit and
// undo semantics are precise rather than pretending a set of filesystem writes
// is atomically transactional.
type MutationKind string

const (
	MutationCreate  MutationKind = "create"
	MutationReplace MutationKind = "replace"
	MutationDelete  MutationKind = "delete"
)

// MutationStatus is the projected lifecycle of a mutation proposal.
type MutationStatus string

const (
	MutationProposed   MutationStatus = "proposed"
	MutationApproved   MutationStatus = "approved"
	MutationCommitting MutationStatus = "committing"
	MutationCommitted  MutationStatus = "committed"
	MutationUndoing    MutationStatus = "undoing"
	MutationUndone     MutationStatus = "undone"
	MutationConflicted MutationStatus = "conflicted"
	MutationRejected   MutationStatus = "rejected"
)

// CreateMutationProposalRequest describes the intended final state of one file.
// Content is required for create/replace (and may explicitly be an empty
// string), and must be omitted for delete.
type CreateMutationProposalRequest struct {
	Kind         MutationKind `json:"kind"`
	Path         string       `json:"path"`
	Content      *string      `json:"content,omitempty"`
	Reason       string       `json:"reason"`
	AgentID      string       `json:"agentId,omitempty"`
	SessionID    string       `json:"sessionId,omitempty"`
	ProvenanceID string       `json:"provenanceId,omitempty"`
}

// ApproveMutationRequest records the identity responsible for authorizing a
// proposal. Approval is intentionally separate from proposal creation.
type ApproveMutationRequest struct {
	ApprovedBy string `json:"approvedBy"`
}

// MutationProposal is the durable proposal and rollback snapshot. before/after
// existence bits distinguish an absent file from a valid empty file.
type MutationProposal struct {
	ID            string         `json:"id"`
	Kind          MutationKind   `json:"kind"`
	Path          string         `json:"path"`
	Reason        string         `json:"reason"`
	AgentID       string         `json:"agentId,omitempty"`
	SessionID     string         `json:"sessionId,omitempty"`
	ProvenanceID  string         `json:"provenanceId,omitempty"`
	Status        MutationStatus `json:"status"`
	BeforeExists  bool           `json:"beforeExists"`
	AfterExists   bool           `json:"afterExists"`
	BeforeHash    string         `json:"beforeHash,omitempty"`
	AfterHash     string         `json:"afterHash,omitempty"`
	BeforeContent string         `json:"beforeContent,omitempty"`
	AfterContent  string         `json:"afterContent,omitempty"`
	Diff          string         `json:"diff"`
	ApprovedBy    string         `json:"approvedBy,omitempty"`
	LastError     string         `json:"lastError,omitempty"`
	CreatedAt     string         `json:"createdAt"`
	UpdatedAt     string         `json:"updatedAt"`
	ApprovedAt    string         `json:"approvedAt,omitempty"`
	CommittedAt   string         `json:"committedAt,omitempty"`
	UndoneAt      string         `json:"undoneAt,omitempty"`
}

// MutationResult returns a proposal plus non-fatal projection/index warnings.
type MutationResult struct {
	Proposal MutationProposal `json:"proposal"`
	Warnings []string         `json:"warnings,omitempty"`
}

// MutationProposalFilter scopes proposal listing.
type MutationProposalFilter struct {
	Status    MutationStatus `json:"status,omitempty"`
	AgentID   string         `json:"agentId,omitempty"`
	SessionID string         `json:"sessionId,omitempty"`
	Limit     int            `json:"limit,omitempty"`
}
