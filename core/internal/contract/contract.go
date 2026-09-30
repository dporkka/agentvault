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
	ID         int     `json:"id"`
	FromNoteID string  `json:"fromNoteId"`
	ToNoteID   *string `json:"toNoteId"`
	RawTarget  string  `json:"rawTarget"`
	LinkType   string  `json:"linkType"`
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
	ID          int     `json:"id"`
	Role        string  `json:"role"`
	Content     string  `json:"content"`
	SourcesJSON *string `json:"sourcesJson,omitempty"`
	CreatedAt   string  `json:"createdAt"`
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

// ProposePromotionRequest is the body for POST /promotions.
type ProposePromotionRequest struct {
	AgentID              string   `json:"agentId"`
	TargetKind           string   `json:"targetKind"`
	Candidate            string   `json:"candidate"`
	Rationale            string   `json:"rationale,omitempty"`
	SourceRunIDs         []string `json:"sourceRunIds,omitempty"`
	SourceObservationIDs []string `json:"sourceObservationIds,omitempty"`
	SourceEvaluationIDs  []string `json:"sourceEvaluationIds,omitempty"`
	SupersedesNoteID     string   `json:"supersedesNoteId,omitempty"`
}

// ReviewPromotionRequest is the body for POST /promotions/{id}/review.
type ReviewPromotionRequest struct {
	Decision string `json:"decision"`
	Reviewer string `json:"reviewer"`
	Note     string `json:"note,omitempty"`
}

// CommitPromotionRequest is the body for POST /promotions/{id}/commit.
type CommitPromotionRequest struct {
	TargetNoteID string `json:"targetNoteId"`
}

// CreateEvaluationDatasetRequest is the body for POST /evaluation-datasets.
type CreateEvaluationDatasetRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	AgentID     string `json:"agentId,omitempty"`
}

// CreateEvaluationCaseRequest is the body for POST /evaluation-datasets/{id}/cases.
type CreateEvaluationCaseRequest struct {
	Name     string                 `json:"name"`
	Input    map[string]interface{} `json:"input"`
	Expected map[string]interface{} `json:"expected,omitempty"`
	Tags     []string               `json:"tags,omitempty"`
}

// CreateExperimentRequest is the body for POST /experiments.
type CreateExperimentRequest struct {
	DatasetID     string                 `json:"datasetId"`
	Name          string                 `json:"name"`
	AgentID       string                 `json:"agentId"`
	AgentRevision int                    `json:"agentRevision"`
	Status        string                 `json:"status,omitempty"`
	Config        map[string]interface{} `json:"config,omitempty"`
}

// CreateExperimentResultRequest is the body for POST /experiments/{id}/results.
type CreateExperimentResultRequest struct {
	CaseID   string                 `json:"caseId"`
	RunID    string                 `json:"runId,omitempty"`
	Score    *float64               `json:"score,omitempty"`
	Label    string                 `json:"label,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ContextCompileRequest is the body for POST /agents/{id}/context.
type ContextCompileRequest struct {
	Task                    string   `json:"task,omitempty"`
	ConversationID          string   `json:"conversationId,omitempty"`
	RetrievedNoteIDs        []string `json:"retrievedNoteIds,omitempty"`
	ArtifactNoteIDs         []string `json:"artifactNoteIds,omitempty"`
	MaxConversationMessages int      `json:"maxConversationMessages,omitempty"`
}

// ContextSection is one ordered source included in a compiled context.
type ContextSection struct {
	Kind       string `json:"kind"`
	SourceID   string `json:"sourceId"`
	SourcePath string `json:"sourcePath"`
	Title      string `json:"title"`
	Content    string `json:"content"`
}

// ContextReferenceIssue records a manifest or request reference that could not be resolved.
type ContextReferenceIssue struct {
	Kind     string `json:"kind"`
	SourceID string `json:"sourceId"`
	Reason   string `json:"reason"`
}

// ContextSnapshot is the immutable, provenance-carrying result of context compilation.
type ContextSnapshot struct {
	Hash               string                  `json:"hash"`
	AgentID            string                  `json:"agentId"`
	AgentRevision      int                     `json:"agentRevision"`
	AgentTitle         string                  `json:"agentTitle"`
	Task               string                  `json:"task"`
	ConversationID     string                  `json:"conversationId"`
	KnowledgeScopes    []string                `json:"knowledgeScopes"`
	ArtifactScopes     []string                `json:"artifactScopes"`
	ConversationScopes []string                `json:"conversationScopes"`
	CapabilityRefs     []string                `json:"capabilityRefs"`
	ContextPolicyRef   string                  `json:"contextPolicyRef"`
	Sections           []ContextSection        `json:"sections"`
	Unresolved         []ContextReferenceIssue `json:"unresolved"`
	Text               string                  `json:"text"`
}

// CreateRunRequest is the body for POST /runs.
type CreateRunRequest struct {
	AgentName          string                 `json:"agentName"`
	AgentID            string                 `json:"agentId,omitempty"`
	AgentRevision      int                    `json:"agentRevision,omitempty"`
	Task               string                 `json:"task"`
	Status             string                 `json:"status,omitempty"`
	ConversationID     string                 `json:"conversationId,omitempty"`
	ContextHash        string                 `json:"contextHash,omitempty"`
	Input              map[string]interface{} `json:"input,omitempty"`
	Output             map[string]interface{} `json:"output,omitempty"`
	CapabilitySnapshot map[string]interface{} `json:"capabilitySnapshot,omitempty"`
	RuntimeMetadata    map[string]interface{} `json:"runtimeMetadata,omitempty"`
	StartedAt          string                 `json:"startedAt,omitempty"`
	EndedAt            string                 `json:"endedAt,omitempty"`
	FilesChanged       []string               `json:"filesChanged,omitempty"`
}

// RunRecord is persisted execution evidence for one agent runtime invocation.
type RunRecord struct {
	ID                 string                 `json:"id"`
	AgentName          string                 `json:"agentName"`
	AgentID            string                 `json:"agentId"`
	AgentRevision      int                    `json:"agentRevision"`
	Task               string                 `json:"task"`
	Status             string                 `json:"status"`
	ConversationID     string                 `json:"conversationId"`
	ContextHash        string                 `json:"contextHash"`
	Input              map[string]interface{} `json:"input"`
	Output             map[string]interface{} `json:"output"`
	CapabilitySnapshot map[string]interface{} `json:"capabilitySnapshot"`
	RuntimeMetadata    map[string]interface{} `json:"runtimeMetadata"`
	StartedAt          string                 `json:"startedAt"`
	EndedAt            string                 `json:"endedAt"`
	FilesChanged       []string               `json:"filesChanged"`
	CreatedAt          string                 `json:"createdAt"`
}

// RunObservation is one structured observation attached to a run audit.
type RunObservation struct {
	ID                  string                 `json:"id"`
	RunID               string                 `json:"runId"`
	ParentObservationID string                 `json:"parentObservationId"`
	Kind                string                 `json:"kind"`
	Name                string                 `json:"name"`
	Status              string                 `json:"status"`
	Input               map[string]interface{} `json:"input"`
	Output              map[string]interface{} `json:"output"`
	Evidence            map[string]interface{} `json:"evidence"`
	StartedAt           string                 `json:"startedAt"`
	EndedAt             string                 `json:"endedAt"`
	CreatedAt           string                 `json:"createdAt"`
}

// RunEvaluation is evaluation evidence attached to a run or observation.
type RunEvaluation struct {
	ID            string                 `json:"id"`
	RunID         string                 `json:"runId"`
	ObservationID string                 `json:"observationId"`
	Evaluator     string                 `json:"evaluator"`
	Name          string                 `json:"name"`
	Score         *float64               `json:"score"`
	Label         string                 `json:"label"`
	Rationale     string                 `json:"rationale"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     string                 `json:"createdAt"`
}

// RunAudit joins a run to its exact immutable context and downstream evidence.
type RunAudit struct {
	Run          RunRecord        `json:"run"`
	Context      *ContextSnapshot `json:"context"`
	Observations []RunObservation `json:"observations"`
	Evaluations  []RunEvaluation  `json:"evaluations"`
}

// RunLearningCandidateRequest creates a reviewable promotion proposal from one run.
// Agent identity is derived server-side from the run and cannot be supplied here.
type RunLearningCandidateRequest struct {
	TargetKind           string   `json:"targetKind"`
	Candidate            string   `json:"candidate"`
	Rationale            string   `json:"rationale,omitempty"`
	SourceObservationIDs []string `json:"sourceObservationIds,omitempty"`
	SourceEvaluationIDs  []string `json:"sourceEvaluationIds,omitempty"`
	SupersedesNoteID     string   `json:"supersedesNoteId,omitempty"`
}


// LearningSignal is one explicit failure signal surfaced from run evidence.
type LearningSignal struct {
	Kind          string   `json:"kind"`
	ID            string   `json:"id"`
	ObservationID string   `json:"observationId"`
	Name          string   `json:"name"`
	Status        string   `json:"status"`
	Label         string   `json:"label"`
	Score         *float64 `json:"score"`
	Rationale     string   `json:"rationale"`
}

// LearningRecommendation is a deterministic, read-only set of evidence inputs
// that a caller may use when deciding whether and how to propose learning.
type LearningRecommendation struct {
	RunID                string           `json:"runId"`
	AgentID              string           `json:"agentId"`
	AgentRevision        int              `json:"agentRevision"`
	Eligible             bool             `json:"eligible"`
	SupportLevel         string           `json:"supportLevel"`
	EvidenceCount        int              `json:"evidenceCount"`
	SuggestedTargetKind  string           `json:"suggestedTargetKind"`
	ReasonCodes          []string         `json:"reasonCodes"`
	SourceObservationIDs []string         `json:"sourceObservationIds"`
	SourceEvaluationIDs  []string         `json:"sourceEvaluationIds"`
	ContextMemoryRefs    []string         `json:"contextMemoryRefs"`
	SupersedesNoteIDs    []string         `json:"supersedesNoteIds"`
	Signals              []LearningSignal `json:"signals"`
}
