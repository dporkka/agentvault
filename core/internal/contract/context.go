package contract

// CompileContextRequest asks AgentVault to assemble a deterministic, evidence-
// backed context bundle for one agent task.
type CompileContextRequest struct {
	Task        string   `json:"task"`
	Project     string   `json:"project,omitempty"`
	AgentID     string   `json:"agentId,omitempty"`
	SessionID   string   `json:"sessionId,omitempty"`
	ObjectIDs   []string `json:"objectIds,omitempty"`
	TokenBudget int      `json:"tokenBudget,omitempty"`
	MaxItems    int      `json:"maxItems,omitempty"`
	AsOf        string   `json:"asOf,omitempty"`
}

// ContextBundle is the compiled, model-agnostic context payload. estimatedTokens
// uses AgentVault's deterministic approximation and never exceeds tokenBudget.
type ContextBundle struct {
	Version         string            `json:"version"`
	Task            string            `json:"task"`
	Project         string            `json:"project,omitempty"`
	AgentID         string            `json:"agentId,omitempty"`
	SessionID       string            `json:"sessionId,omitempty"`
	AsOf            string            `json:"asOf"`
	TokenBudget     int               `json:"tokenBudget"`
	EstimatedTokens int               `json:"estimatedTokens"`
	Truncated       bool              `json:"truncated"`
	Items           []ContextItem     `json:"items"`
	Stats           ContextBundleStats `json:"stats"`
}

// ContextBundleStats explains which sources contributed to a bundle.
type ContextBundleStats struct {
	Candidates int            `json:"candidates"`
	Included   int            `json:"included"`
	Dropped    int            `json:"dropped"`
	ByKind     map[string]int `json:"byKind"`
}

// ContextItem is one ranked unit of evidence/context.
type ContextItem struct {
	Kind            string             `json:"kind"`
	ID              string             `json:"id"`
	Title           string             `json:"title,omitempty"`
	Content         string             `json:"content"`
	Path            string             `json:"path,omitempty"`
	Score           float64            `json:"score"`
	EstimatedTokens int                `json:"estimatedTokens"`
	ObjectIDs       []string           `json:"objectIds,omitempty"`
	Provenance      *ContextProvenance `json:"provenance,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// ContextProvenance is the compact provenance view carried beside compiled
// context so consumers do not have to perform a second lookup to assess trust.
type ContextProvenance struct {
	ID         string               `json:"id"`
	SourceType string               `json:"sourceType"`
	SourceID   string               `json:"sourceId,omitempty"`
	AgentID    string               `json:"agentId,omitempty"`
	SessionID  string               `json:"sessionId,omitempty"`
	Model      string               `json:"model,omitempty"`
	Confidence float64              `json:"confidence"`
	ObservedAt string               `json:"observedAt"`
	Evidence   []ProvenanceEvidence `json:"evidence,omitempty"`
}
