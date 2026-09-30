package agentstate

import (
	"fmt"
	"time"
)

// AgentManifest is the canonical, file-backed aggregate describing an agent.
// References point at other AgentVault objects; the manifest does not duplicate
// memory, knowledge, artifacts, conversations, or capability definitions.
type AgentManifest struct {
	ID                 string
	Name               string
	Purpose            string
	Revision           int
	IdentityRef        string
	MemoryRefs         []string
	KnowledgeScopes    []string
	ArtifactScopes     []string
	ConversationScopes []string
	CapabilityRefs     []string
	ContextPolicyRef   string
}

func (m AgentManifest) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("agent id is required")
	}
	if m.Name == "" {
		return fmt.Errorf("agent name is required")
	}
	if m.Revision < 1 {
		return fmt.Errorf("agent revision must be at least 1")
	}
	return nil
}

type RunStatus string

const (
	RunRunning   RunStatus = "running"
	RunSucceeded RunStatus = "succeeded"
	RunFailed    RunStatus = "failed"
	RunCancelled RunStatus = "cancelled"
)

func (s RunStatus) valid() bool {
	switch s {
	case RunRunning, RunSucceeded, RunFailed, RunCancelled:
		return true
	default:
		return false
	}
}

// Run captures execution evidence for a specific agent revision. It describes
// what happened; AgentVault remains independent from the runtime that executed it.
type Run struct {
	ID                 string
	AgentID            string
	AgentRevision      int
	ConversationID     string
	Status             RunStatus
	StartedAt          time.Time
	EndedAt            *time.Time
	ContextHash        string
	CapabilitySnapshot map[string]any
	RuntimeMetadata    map[string]any
}

func (r Run) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("run id is required")
	}
	if r.AgentID == "" {
		return fmt.Errorf("agent id is required")
	}
	if r.AgentRevision < 1 {
		return fmt.Errorf("agent revision must be at least 1")
	}
	if !r.Status.valid() {
		return fmt.Errorf("unknown run status %q", r.Status)
	}
	return nil
}

// Evaluation is evidence about the quality of a run or one of its observations.
type Evaluation struct {
	ID            string
	RunID         string
	ObservationID string
	Evaluator     string
	Name          string
	Score         *float64
	Label         string
	Rationale     string
	Metadata      map[string]any
}

func (e Evaluation) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("evaluation id is required")
	}
	if e.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	if e.Evaluator == "" {
		return fmt.Errorf("evaluator is required")
	}
	if e.Name == "" {
		return fmt.Errorf("evaluation name is required")
	}
	if e.Score == nil && e.Label == "" {
		return fmt.Errorf("evaluation requires a score or label")
	}
	return nil
}

type ObservationKind string

const (
	ObservationContextCompile ObservationKind = "context.compile"
	ObservationRetrieval      ObservationKind = "retrieval"
	ObservationGeneration     ObservationKind = "generation"
	ObservationTool           ObservationKind = "tool"
	ObservationArtifactWrite  ObservationKind = "artifact.write"
	ObservationEvent          ObservationKind = "event"
)

func (k ObservationKind) valid() bool {
	switch k {
	case ObservationContextCompile, ObservationRetrieval, ObservationGeneration,
		ObservationTool, ObservationArtifactWrite, ObservationEvent:
		return true
	default:
		return false
	}
}

// Observation is one structured step inside a Run. ParentObservationID allows
// nesting without turning the evidence model into an execution engine.
type Observation struct {
	ID                  string
	RunID               string
	ParentObservationID string
	Kind                ObservationKind
	Name                string
	Status              string
	Input               map[string]any
	Output              map[string]any
	Evidence            map[string]any
	StartedAt           time.Time
	EndedAt             *time.Time
}

func (o Observation) Validate() error {
	if o.ID == "" {
		return fmt.Errorf("observation id is required")
	}
	if o.RunID == "" {
		return fmt.Errorf("run id is required")
	}
	if !o.Kind.valid() {
		return fmt.Errorf("unknown observation kind %q", o.Kind)
	}
	if o.Name == "" {
		return fmt.Errorf("observation name is required")
	}
	return nil
}

type PromotionTargetKind string

const (
	PromotionMemory    PromotionTargetKind = "memory"
	PromotionKnowledge PromotionTargetKind = "knowledge"
)

type PromotionStatus string

const (
	PromotionProposed   PromotionStatus = "proposed"
	PromotionApproved   PromotionStatus = "approved"
	PromotionRejected   PromotionStatus = "rejected"
	PromotionCommitted  PromotionStatus = "committed"
	PromotionSuperseded PromotionStatus = "superseded"
)

// PromotionRecord records lineage and review state. TargetNoteID points at the
// canonical Markdown note after commit; promoted content is never a second
// hidden memory store.
type PromotionRecord struct {
	ID                   string
	AgentID              string
	TargetKind           PromotionTargetKind
	Status               PromotionStatus
	Candidate            string
	Rationale            string
	SourceRunIDs         []string
	SourceObservationIDs []string
	SourceEvaluationIDs  []string
	TargetNoteID         string
	SupersedesNoteID     string
}

func (p PromotionRecord) Validate() error {
	if p.ID == "" {
		return fmt.Errorf("promotion id is required")
	}
	if p.AgentID == "" {
		return fmt.Errorf("agent id is required")
	}
	if p.TargetKind != PromotionMemory && p.TargetKind != PromotionKnowledge {
		return fmt.Errorf("unknown promotion target kind %q", p.TargetKind)
	}
	switch p.Status {
	case PromotionProposed, PromotionApproved, PromotionRejected, PromotionCommitted, PromotionSuperseded:
	default:
		return fmt.Errorf("unknown promotion status %q", p.Status)
	}
	if p.Candidate == "" {
		return fmt.Errorf("promotion candidate is required")
	}
	if len(p.SourceRunIDs)+len(p.SourceObservationIDs)+len(p.SourceEvaluationIDs) == 0 {
		return fmt.Errorf("promotion requires at least one evidence source")
	}
	if (p.Status == PromotionCommitted || p.Status == PromotionSuperseded) && p.TargetNoteID == "" {
		return fmt.Errorf("%s promotion requires target note id", p.Status)
	}
	return nil
}

func ValidatePromotionTransition(from, to PromotionStatus) error {
	switch {
	case from == PromotionProposed && (to == PromotionApproved || to == PromotionRejected):
		return nil
	case from == PromotionApproved && to == PromotionCommitted:
		return nil
	case from == PromotionCommitted && to == PromotionSuperseded:
		return nil
	default:
		return fmt.Errorf("invalid promotion transition %s -> %s", from, to)
	}
}

// EvaluationDataset groups stable cases used to compare agent revisions or runtime configurations.
type EvaluationDataset struct {
	ID          string
	Name        string
	Description string
	AgentID     string
}

func (d EvaluationDataset) Validate() error {
	if d.ID == "" {
		return fmt.Errorf("dataset id is required")
	}
	if d.Name == "" {
		return fmt.Errorf("dataset name is required")
	}
	return nil
}

// EvaluationCase is a single reproducible input and optional expected outcome.
type EvaluationCase struct {
	ID        string
	DatasetID string
	Name      string
	Input     map[string]any
	Expected  map[string]any
	Tags      []string
}

func (c EvaluationCase) Validate() error {
	if c.ID == "" {
		return fmt.Errorf("evaluation case id is required")
	}
	if c.DatasetID == "" {
		return fmt.Errorf("dataset id is required")
	}
	if c.Name == "" {
		return fmt.Errorf("evaluation case name is required")
	}
	if c.Input == nil {
		return fmt.Errorf("evaluation case input is required")
	}
	return nil
}

type ExperimentStatus string

const (
	ExperimentPlanned   ExperimentStatus = "planned"
	ExperimentRunning   ExperimentStatus = "running"
	ExperimentCompleted ExperimentStatus = "completed"
	ExperimentFailed    ExperimentStatus = "failed"
	ExperimentCancelled ExperimentStatus = "cancelled"
)

func (s ExperimentStatus) valid() bool {
	switch s {
	case ExperimentPlanned, ExperimentRunning, ExperimentCompleted, ExperimentFailed, ExperimentCancelled:
		return true
	default:
		return false
	}
}

// Experiment records an externally executed evaluation against one immutable agent revision.
type Experiment struct {
	ID            string
	DatasetID     string
	Name          string
	AgentID       string
	AgentRevision int
	Status        ExperimentStatus
	Config        map[string]any
}

func (e Experiment) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("experiment id is required")
	}
	if e.DatasetID == "" {
		return fmt.Errorf("dataset id is required")
	}
	if e.Name == "" {
		return fmt.Errorf("experiment name is required")
	}
	if e.AgentID == "" {
		return fmt.Errorf("agent id is required")
	}
	if e.AgentRevision < 1 {
		return fmt.Errorf("agent revision must be at least 1")
	}
	if !e.Status.valid() {
		return fmt.Errorf("unknown experiment status %q", e.Status)
	}
	return nil
}

// ExperimentResult links one dataset case to the evidence produced by an external run.
type ExperimentResult struct {
	ExperimentID string
	CaseID       string
	RunID        string
	Score        *float64
	Label        string
	Metadata     map[string]any
}

func (r ExperimentResult) Validate() error {
	if r.ExperimentID == "" {
		return fmt.Errorf("experiment id is required")
	}
	if r.CaseID == "" {
		return fmt.Errorf("case id is required")
	}
	if r.Score == nil && r.Label == "" {
		return fmt.Errorf("experiment result requires a score or label")
	}
	return nil
}


// ActionIntent is the pre-dispatch evidence for one externally executed logical
// action. AgentVault records the intent and authority binding but does not
// execute the action itself.
type ActionIntent struct {
	ID                  string
	RunID               string
	ObservationID       string
	OperationID         string
	Action              string
	CapabilityRef       string
	AuthorityHash       string
	InputHash           string
	Metadata            map[string]any
	CreatedAt           time.Time
}

func (i ActionIntent) Validate() error {
	if i.ID == "" {
		return fmt.Errorf("action intent id is required")
	}
	if i.RunID == "" {
		return fmt.Errorf("action intent run id is required")
	}
	if i.OperationID == "" {
		return fmt.Errorf("action intent operation id is required")
	}
	if i.Action == "" {
		return fmt.Errorf("action intent action is required")
	}
	if i.CapabilityRef == "" {
		return fmt.Errorf("action intent capability ref is required")
	}
	if i.AuthorityHash == "" {
		return fmt.Errorf("action intent authority hash is required")
	}
	if i.InputHash == "" {
		return fmt.Errorf("action intent input hash is required")
	}
	return nil
}

type ActionReceiptStatus string

const (
	ActionCompleted     ActionReceiptStatus = "completed"
	ActionFailed        ActionReceiptStatus = "failed"
	ActionIndeterminate ActionReceiptStatus = "indeterminate"
)

func (s ActionReceiptStatus) valid() bool {
	switch s {
	case ActionCompleted, ActionFailed, ActionIndeterminate:
		return true
	default:
		return false
	}
}

// ActionReceipt is the immutable terminal evidence for one ActionIntent.
type ActionReceipt struct {
	ID                string
	IntentID          string
	Status            ActionReceiptStatus
	ResultHash        string
	ExternalReceiptID string
	ErrorCode         string
	ErrorMessage      string
	Metadata          map[string]any
	CompletedAt       time.Time
}

func (r ActionReceipt) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("action receipt id is required")
	}
	if r.IntentID == "" {
		return fmt.Errorf("action receipt intent id is required")
	}
	if !r.Status.valid() {
		return fmt.Errorf("unknown action receipt status %q", r.Status)
	}
	switch r.Status {
	case ActionCompleted:
		if r.ResultHash == "" {
			return fmt.Errorf("completed action receipt requires result hash")
		}
	case ActionFailed, ActionIndeterminate:
		if r.ErrorCode == "" && r.ErrorMessage == "" {
			return fmt.Errorf("%s action receipt requires an error code or message", r.Status)
		}
	}
	return nil
}
