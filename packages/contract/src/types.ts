// Canonical TypeScript types for the AgentVault HTTP API. These mirror the
// exact JSON the Go server emits (see docs/API_CONTRACT.md) and are the
// single source of truth for the web, browser-extension, and mobile clients.
// The Wails desktop frontend reuses them for the Wails bridge, and the
// shared `core/internal/contract` Go package is the server-side twin.

export interface HealthResponse {
  status: string;
  vault: string;
  version: string;
}

// GET /auth/verify
export interface AuthVerifyResponse {
  status: string;
  server: string;
  version: string;
  hasToken: boolean;
  tokenValid: boolean;
}

// GET /vault/status
export interface VaultStatus {
  path: string;
  isVault: boolean;
  noteCount: number;
  version: string;
}

// POST /vault/index
export interface IndexError {
  path: string;
  error: string;
}

export interface IndexResult {
  scanned: number;
  added: number;
  updated: number;
  removed: number;
  skipped: number;
  errors: IndexError[] | null;
  chunksAdded: number;
  embedErrors: number;
  /** Indexing duration in nanoseconds. */
  duration: number;
}

export interface IndexOptions {
  force?: boolean;
  rebuild?: boolean;
  path?: string;
  embed?: boolean;
}

// GET /search, /recent, /stale — same shape across the three.
export interface SearchResult {
  id: string;
  title: string;
  path: string;
  type: string;
  project: string;
  status: string;
  tags: string[];
  snippet: string;
  score: number;
  updatedAt: string;
}

// GET /notes/{id}
export interface NoteDetail {
  id: string;
  title: string;
  path: string;
  type: string;
  project: string;
  status: string;
  tags: string[];
  content: string;
}

// GET /notes/{id}/links
export interface Link {
  id: number;
  fromNoteId: string;
  toNoteId: string | null;
  rawTarget: string;
  linkType: string;
}

export interface NoteLinks {
  backlinks: Link[];
  outgoing: Link[];
}



// PUT /notes/{id}
export interface UpdateNoteRequest {
  title?: string;
  content?: string;
  tags?: string[];
  status?: string;
  project?: string;
}

export interface UpdateNoteResponse {
  path: string;
  id: string;
}

// DELETE /notes/{id}
export interface DeleteNoteResponse {
  path: string;
  id: string;
}

// POST /notes
export interface CreateNoteRequest {
  type?: string;
  title: string;
  project?: string;
  tags?: string[];
}

export interface CreateNoteResponse {
  path: string;
  id: string;
}

export interface DeleteNoteResponse {
  path: string;
  id: string;
}

// POST /capture
export interface CaptureRequest {
  type?: string;
  title?: string;
  url?: string;
  text?: string;
  project?: string;
  tags?: string[];
  externalId?: string;
}

export interface CaptureResponse {
  path: string;
}

// POST /ask
export interface AskSource {
  id: string;
  path: string;
  title: string;
  excerpt?: string;
}

export interface AskRequest {
  question: string;
}

export interface AskResponse {
  answer: string;
  sources: AskSource[];
  confidence: string;
  caveats?: string[];
  missingInfo?: string;
  suggestedActions?: string[];
}

// GET /projects
export type Projects = string[];

// GET /git/status
export interface GitModifiedFile {
  path: string;
  status: string;
  staged: boolean;
}

export interface GitStatus {
  isGitRepo: boolean;
  branch: string;
  clean: boolean;
  aheadBehind: string;
  modifiedFiles: GitModifiedFile[];
  untrackedFiles: string[];
}

// Parameters accepted by /search. All fields are optional; the client just
// omits the ones it doesn't want.
export interface SearchParams {
  q?: string;
  type?: string;
  project?: string;
  tag?: string;
  status?: string;
  pinned?: boolean;
  limit?: number;
  offset?: number;
  vector?: boolean;
  hybridWeight?: number;
  topk?: number;
}

export interface RecentParams {
  limit?: number;
}

export interface StaleParams {
  days?: number;
  limit?: number;
}

// GET /graph, GET /graph/neighbors
export interface GraphNode {
  id: string;
  title: string;
  type: string;
  project: string;
}

export interface GraphEdge {
  fromId: string;
  toId: string;
  linkType: string;
}

export interface Graph {
  nodes: GraphNode[];
  edges: GraphEdge[];
}

// Conversations
export interface Conversation {
  id: string;
  title: string;
  createdAt: string;
  updatedAt: string;
  messages?: ConversationMessage[];
}

export interface ConversationMessage {
  id: number;
  role: string;
  content: string;
  sourcesJson?: string | null;
  createdAt: string;
}

export interface CreateConversationRequest {
  title?: string;
}

export interface ConversationAskRequest {
  question: string;
}


export type PromotionStatus =
  | 'proposed'
  | 'approved'
  | 'rejected'
  | 'committed'
  | 'superseded';

export interface Promotion {
  id: string;
  agentId: string;
  targetKind: 'memory' | 'knowledge';
  status: PromotionStatus;
  candidate: string;
  rationale: string;
  sourceRunIds: string[];
  sourceObservationIds: string[];
  sourceEvaluationIds: string[];
  targetNoteId: string;
  supersedesNoteId: string;
  createdAt: string;
  reviewedAt: string;
  reviewedBy: string;
  reviewNote: string;
  committedAt: string;
}

export interface PromotionParams {
  status?: PromotionStatus | 'all';
  agentId?: string;
  limit?: number;
}

export interface EvaluationDataset {
  id: string;
  name: string;
  description: string;
  agentId: string;
  createdAt: string;
}

export interface EvaluationCase {
  id: string;
  datasetId: string;
  name: string;
  input: Record<string, unknown>;
  expected: Record<string, unknown> | null;
  tags: string[];
  createdAt: string;
}

export interface EvaluationDatasetDetail extends EvaluationDataset {
  cases: EvaluationCase[];
}

export interface Experiment {
  id: string;
  datasetId: string;
  name: string;
  agentId: string;
  agentRevision: number;
  status: 'planned' | 'running' | 'completed' | 'failed' | 'cancelled';
  config: Record<string, unknown>;
  createdAt: string;
  completedAt: string;
}

export interface ExperimentResult {
  experimentId: string;
  caseId: string;
  runId: string;
  score: number | null;
  label: string;
  metadata: Record<string, unknown>;
  createdAt: string;
}

export interface ExperimentDetail extends Experiment {
  results: ExperimentResult[];
}


export interface ProposePromotionRequest {
  agentId: string;
  targetKind: 'memory' | 'knowledge';
  candidate: string;
  rationale?: string;
  sourceRunIds?: string[];
  sourceObservationIds?: string[];
  sourceEvaluationIds?: string[];
  supersedesNoteId?: string;
}

export interface ReviewPromotionRequest {
  decision: 'approve' | 'reject';
  reviewer: string;
  note?: string;
}

export interface CommitPromotionRequest {
  targetNoteId: string;
}

export interface CreateEvaluationDatasetRequest {
  name: string;
  description?: string;
  agentId?: string;
}

export interface CreateEvaluationCaseRequest {
  name: string;
  input: Record<string, unknown>;
  expected?: Record<string, unknown>;
  tags?: string[];
}

export interface CreateExperimentRequest {
  datasetId: string;
  name: string;
  agentId: string;
  agentRevision: number;
  status?: Experiment['status'];
  config?: Record<string, unknown>;
}

export interface CreateExperimentResultRequest {
  caseId: string;
  runId?: string;
  score?: number;
  label?: string;
  metadata?: Record<string, unknown>;
}


export interface ContextCompileRequest {
  task?: string;
  conversationId?: string;
  retrievedNoteIds?: string[];
  artifactNoteIds?: string[];
  maxConversationMessages?: number;
}

export type ContextSectionKind =
  | 'agent'
  | 'identity'
  | 'context_policy'
  | 'memory'
  | 'task'
  | 'knowledge'
  | 'conversation'
  | 'artifact';

export interface ContextSection {
  kind: ContextSectionKind;
  sourceId: string;
  sourcePath: string;
  title: string;
  content: string;
}

export interface ContextReferenceIssue {
  kind: ContextSectionKind;
  sourceId: string;
  reason: string;
}

export interface ContextSnapshot {
  hash: string;
  agentId: string;
  agentRevision: number;
  agentTitle: string;
  task: string;
  conversationId: string;
  knowledgeScopes: string[];
  artifactScopes: string[];
  conversationScopes: string[];
  capabilityRefs: string[];
  contextPolicyRef: string;
  sections: ContextSection[];
  unresolved: ContextReferenceIssue[];
  text: string;
}


export type RunStatus = 'running' | 'succeeded' | 'failed' | 'cancelled';

export interface CreateRunRequest {
  agentName: string;
  agentId?: string;
  agentRevision?: number;
  task: string;
  status?: RunStatus;
  conversationId?: string;
  contextHash?: string;
  input?: Record<string, unknown>;
  output?: Record<string, unknown>;
  capabilitySnapshot?: Record<string, unknown>;
  runtimeMetadata?: Record<string, unknown>;
  startedAt?: string;
  endedAt?: string;
  filesChanged?: string[];
}

export interface RunRecord {
  id: string;
  agentName: string;
  agentId: string;
  agentRevision: number;
  task: string;
  status: RunStatus;
  conversationId: string;
  contextHash: string;
  input: Record<string, unknown>;
  output: Record<string, unknown>;
  capabilitySnapshot: Record<string, unknown>;
  runtimeMetadata: Record<string, unknown>;
  startedAt: string;
  endedAt: string;
  filesChanged: string[];
  createdAt: string;
}

export interface RunObservation {
  id: string;
  runId: string;
  parentObservationId: string;
  kind: 'context.compile' | 'retrieval' | 'generation' | 'tool' | 'artifact.write' | 'event';
  name: string;
  status: string;
  input: Record<string, unknown>;
  output: Record<string, unknown>;
  evidence: Record<string, unknown>;
  startedAt: string;
  endedAt: string;
  createdAt: string;
}

export interface RunEvaluation {
  id: string;
  runId: string;
  observationId: string;
  evaluator: string;
  name: string;
  score: number | null;
  label: string;
  rationale: string;
  metadata: Record<string, unknown>;
  createdAt: string;
}

export interface RunAudit {
  run: RunRecord;
  context: ContextSnapshot | null;
  observations: RunObservation[];
  evaluations: RunEvaluation[];
}


export interface RunLearningCandidateRequest {
  targetKind: 'memory' | 'knowledge';
  candidate: string;
  rationale?: string;
  sourceObservationIds?: string[];
  sourceEvaluationIds?: string[];
  supersedesNoteId?: string;
}


export type LearningSupportLevel = 'none' | 'weak' | 'moderate' | 'strong';
export type LearningSignalKind = 'observation' | 'evaluation';

export interface LearningSignal {
  kind: LearningSignalKind;
  id: string;
  observationId: string;
  name: string;
  status: string;
  label: string;
  score: number | null;
  rationale: string;
}

export interface LearningRecommendation {
  runId: string;
  agentId: string;
  agentRevision: number;
  eligible: boolean;
  supportLevel: LearningSupportLevel;
  evidenceCount: number;
  suggestedTargetKind: '' | 'memory' | 'knowledge';
  reasonCodes: string[];
  sourceObservationIds: string[];
  sourceEvaluationIds: string[];
  contextMemoryRefs: string[];
  supersedesNoteIds: string[];
  signals: LearningSignal[];
}
