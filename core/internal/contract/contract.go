// Package contract holds the canonical JSON types shared between the
// AgentVault HTTP server, the Wails desktop app, and any other Go client
// of the same vault primitives. Types here carry the same camelCase
// `json` tags the HTTP API emits, so an HTTP client and a Go bridge
// client see identical shapes.
package contract

import "time"

// SearchResult is the shape of a single hit returned by /search, /recent,
// and /stale. It is the persisted-and-serialized form of a vault note's
// metadata plus a search-time snippet/score.
type SearchResult struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Path      string   `json:"path"`
	Type      string   `json:"type"`
	Project   string   `json:"project"`
	Status    string   `json:"status"`
	Tags      []string `json:"tags"`
	Snippet   string   `json:"snippet"`
	Score     float64  `json:"score"`
	UpdatedAt string   `json:"updatedAt"`
}

// NoteDetail is the shape of the /notes/{id} response. It carries the same
// metadata fields as a search hit plus the raw file body so a reader view
// can render the note without a second request.
type NoteDetail struct {
	ID      string   `json:"id"`
	Title   string   `json:"title"`
	Path    string   `json:"path"`
	Type    string   `json:"type"`
	Project string   `json:"project"`
	Status  string   `json:"status"`
	Tags    []string `json:"tags"`
	Content string   `json:"content"`
}

// IndexResult is the body returned by POST /vault/index. duration is the
// Go time.Duration serialized as integer nanoseconds.
type IndexResult struct {
	Scanned     int          `json:"scanned"`
	Added       int          `json:"added"`
	Updated     int          `json:"updated"`
	Removed     int          `json:"removed"`
	Skipped     int          `json:"skipped"`
	Errors      []IndexError `json:"errors"`
	ChunksAdded int          `json:"chunksAdded"`
	EmbedErrors int          `json:"embedErrors"`
	// Duration is the wall-clock indexing time serialized as integer nanoseconds (time.Duration).
	Duration time.Duration `json:"duration"`
}

// IndexError records a single file that failed during an indexing run.
type IndexError struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// Answer is the body returned by POST /ask. caveats, missingInfo, and
// suggestedActions are omitted from the JSON when empty.
type Answer struct {
	Answer           string   `json:"answer"`
	Sources          []Source `json:"sources"`
	Confidence       string   `json:"confidence"`
	Caveats          []string `json:"caveats,omitempty"`
	MissingInfo      string   `json:"missingInfo,omitempty"`
	SuggestedActions []string `json:"suggestedActions,omitempty"`
}

// Source is a single document the RAG pipeline cited in an Answer.
type Source struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Title   string `json:"title"`
	Excerpt string `json:"excerpt,omitempty"`
}

// VaultStatus is the body returned by GET /vault/status. isVault is true
// when the configured path is a valid AgentVault vault (not "the vault
// is open in the UI"; the desktop app reuses this struct but interprets
// isVault as "the desktop process has loaded a vault").
type VaultStatus struct {
	Path      string `json:"path"`
	IsVault   bool   `json:"isVault"`
	NoteCount int    `json:"noteCount"`
	Watching  bool   `json:"watching"`
	Version   string `json:"version"`
}

// GitStatus is the body returned by GET /git/status. When isGitRepo is
// false the other fields are zero-valued (branch="", clean=true, both
// file arrays empty) and the server returns an empty repo state rather
// than an error.
type GitStatus struct {
	IsGitRepo      bool              `json:"isGitRepo"`
	Branch         string            `json:"branch"`
	Clean          bool              `json:"clean"`
	AheadBehind    string            `json:"aheadBehind"`
	ModifiedFiles  []GitModifiedFile `json:"modifiedFiles"`
	UntrackedFiles []string          `json:"untrackedFiles"`
}

// GitModifiedFile is a single entry in GitStatus.ModifiedFiles.
type GitModifiedFile struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Staged bool   `json:"staged"`
}

// Link represents a link between two notes. It is populated during indexing
// from wiki links and markdown links in note bodies.
type Link struct {
	ID          int     `json:"id"`
	FromNoteID  string  `json:"fromNoteId"`
	ToNoteID    *string `json:"toNoteId"`
	RawTarget   string  `json:"rawTarget"`
	LinkType    string  `json:"linkType"`
}

// NoteLinks groups backlinks and outgoing links for a note.
type NoteLinks struct {
	Backlinks []Link `json:"backlinks"`
	Outgoing  []Link `json:"outgoing"`
}

// UpdateNoteRequest is the body for PUT /notes/{id}. All fields are optional;
// only supplied fields overwrite the existing note's frontmatter or body.
type UpdateNoteRequest struct {
	Title   *string  `json:"title,omitempty"`
	Content *string  `json:"content,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Status  *string  `json:"status,omitempty"`
	Project *string  `json:"project,omitempty"`
}

// UpdateNoteResponse is the body returned after a successful note update.
type UpdateNoteResponse struct {
	Path string `json:"path"`
	ID   string `json:"id"`
}

// GraphNode is a vertex in the knowledge graph returned by GET /graph.
type GraphNode struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Project string `json:"project"`
}

// GraphEdge is a directed edge between two notes returned by GET /graph.
type GraphEdge struct {
	FromID   string `json:"fromId"`
	ToID     string `json:"toId"`
	LinkType string `json:"linkType"`
}

// Graph is a subgraph returned by GET /graph and GET /graph/neighbors.
type Graph struct {
	Nodes []GraphNode `json:"nodes"`
	Edges []GraphEdge `json:"edges"`
}

// Conversation represents a multi-turn AI conversation stored in the vault.
type Conversation struct {
	ID        string                `json:"id"`
	Title     string                `json:"title"`
	CreatedAt string                `json:"createdAt"`
	UpdatedAt string                `json:"updatedAt"`
	Messages  []ConversationMessage `json:"messages,omitempty"`
}

// ConversationMessage is a single message in a conversation.
type ConversationMessage struct {
	ID             int     `json:"id"`
	Role           string  `json:"role"`
	Content        string  `json:"content"`
	SourcesJSON    *string `json:"sourcesJson,omitempty"`
	CreatedAt      string  `json:"createdAt"`
}

// CreateConversationRequest is the body for POST /conversations.
type CreateConversationRequest struct {
	Title string `json:"title,omitempty"`
}

// ConversationAskRequest is the body for POST /conversations/{id}/ask.
type ConversationAskRequest struct {
	Question string `json:"question"`
}

// AnnotateRequest is the body for POST /notes/{id}/annotate.
// Agents use this to add metadata without modifying note content.
type AnnotateRequest struct {
	AgentName string            `json:"agentName,omitempty"`
	Notes     string            `json:"notes,omitempty"`
	Status    string            `json:"status,omitempty"`
	Priority  *int              `json:"priority,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}


// Promotion is an evidence-backed memory or knowledge promotion record.
type Promotion struct {
	ID                   string   `json:"id"`
	AgentID              string   `json:"agentId"`
	TargetKind           string   `json:"targetKind"`
	Status               string   `json:"status"`
	Candidate            string   `json:"candidate"`
	Rationale            string   `json:"rationale"`
	SourceRunIDs         []string `json:"sourceRunIds"`
	SourceObservationIDs []string `json:"sourceObservationIds"`
	SourceEvaluationIDs  []string `json:"sourceEvaluationIds"`
	TargetNoteID         string   `json:"targetNoteId"`
	SupersedesNoteID     string   `json:"supersedesNoteId"`
	CreatedAt            string   `json:"createdAt"`
	ReviewedAt           string   `json:"reviewedAt"`
	ReviewedBy           string   `json:"reviewedBy"`
	ReviewNote           string   `json:"reviewNote"`
	CommittedAt          string   `json:"committedAt"`
}

// EvaluationDataset is the metadata for a reusable evaluation dataset.
type EvaluationDataset struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	AgentID     string `json:"agentId"`
	CreatedAt   string `json:"createdAt"`
}

// EvaluationCase is one stable input/expected-output pair in a dataset.
type EvaluationCase struct {
	ID        string                 `json:"id"`
	DatasetID string                 `json:"datasetId"`
	Name      string                 `json:"name"`
	Input     map[string]interface{} `json:"input"`
	Expected  map[string]interface{} `json:"expected"`
	Tags      []string               `json:"tags"`
	CreatedAt string                 `json:"createdAt"`
}

// EvaluationDatasetDetail returns a dataset together with its cases.
type EvaluationDatasetDetail struct {
	EvaluationDataset
	Cases []EvaluationCase `json:"cases"`
}

// Experiment is an externally executed evaluation run recorded by AgentVault.
type Experiment struct {
	ID            string                 `json:"id"`
	DatasetID     string                 `json:"datasetId"`
	Name          string                 `json:"name"`
	AgentID       string                 `json:"agentId"`
	AgentRevision int                    `json:"agentRevision"`
	Status        string                 `json:"status"`
	Config        map[string]interface{} `json:"config"`
	CreatedAt     string                 `json:"createdAt"`
	CompletedAt   string                 `json:"completedAt"`
}

// ExperimentResult is one case-level result within an experiment.
type ExperimentResult struct {
	ExperimentID string                 `json:"experimentId"`
	CaseID       string                 `json:"caseId"`
	RunID        string                 `json:"runId"`
	Score        *float64               `json:"score"`
	Label        string                 `json:"label"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    string                 `json:"createdAt"`
}

// ExperimentDetail returns an experiment together with all recorded case results.
type ExperimentDetail struct {
	Experiment
	Results []ExperimentResult `json:"results"`
}
