// Typed route table for the AgentVault HTTP API. Each entry binds a
// method+path to its request and response payload types, so call sites
// can do `Endpoint<'POST', '/notes'>` to get a `{ request, response }`
// pair with the right TypeScript types.

import type {
  AskRequest,
  AskResponse,
  AuthVerifyResponse,
  CaptureRequest,
  CaptureResponse,
  CommitPromotionRequest,
  ContextCompileRequest,
  ContextSnapshot,
  CreateEvaluationCaseRequest,
  CreateEvaluationDatasetRequest,
  CreateExperimentRequest,
  CreateExperimentResultRequest,
  CreateNoteRequest,
  CreateNoteResponse,
  UpdateNoteRequest,
  UpdateNoteResponse,
  DeleteNoteResponse,
  GitStatus,
  Graph,
  HealthResponse,
  EvaluationCase,
  EvaluationDataset,
  EvaluationDatasetDetail,
  Experiment,
  ExperimentDetail,
  ExperimentResult,
  IndexOptions,
  IndexResult,
  NoteLinks,
  NoteDetail,
  Projects,
  Promotion,
  PromotionParams,
  ProposePromotionRequest,
  ReviewPromotionRequest,
  RecentParams,
  SearchResult,
  SearchParams,
  StaleParams,
  VaultStatus,
} from './types';

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE';

export interface EndpointDef<Req, Res> {
  method: Method;
  path: string;
  auth: boolean;
  request: Req;
  response: Res;
}

export type NoRequest = never;

// routes is the authoritative list. Anything not here is not a server
// route. Keep this list in sync with `core/internal/api/server.go`'s
export const routes: {
  readonly health: EndpointDef<NoRequest, HealthResponse>;
  readonly authVerify: EndpointDef<NoRequest, AuthVerifyResponse>;
  readonly vaultStatus: EndpointDef<NoRequest, VaultStatus>;
  readonly vaultIndex: EndpointDef<IndexOptions | undefined, IndexResult>;
  readonly search: EndpointDef<SearchParams, SearchResult[]>;
  readonly noteById: EndpointDef<{ id: string }, NoteDetail>;
  readonly noteLinks: EndpointDef<{ id: string }, NoteLinks>;
  readonly updateNote: EndpointDef<UpdateNoteRequest, UpdateNoteResponse>;
  readonly deleteNote: EndpointDef<{ id: string }, DeleteNoteResponse>;
  readonly createNote: EndpointDef<CreateNoteRequest, CreateNoteResponse>;
  readonly capture: EndpointDef<CaptureRequest, CaptureResponse>;
  readonly ask: EndpointDef<AskRequest, AskResponse>;
  readonly projects: EndpointDef<NoRequest, Projects>;
  readonly recent: EndpointDef<RecentParams | undefined, SearchResult[]>;
  readonly stale: EndpointDef<StaleParams | undefined, SearchResult[]>;
  readonly gitStatus: EndpointDef<NoRequest, GitStatus>;
  readonly graph: EndpointDef<{ center: string; depth?: number }, Graph>;
  readonly graphNeighbors: EndpointDef<{ id: string }, Graph>;
  readonly promotions: EndpointDef<PromotionParams | undefined, Promotion[]>;
  readonly proposePromotion: EndpointDef<ProposePromotionRequest, Promotion>;
  readonly reviewPromotion: EndpointDef<ReviewPromotionRequest, Promotion>;
  readonly commitPromotion: EndpointDef<CommitPromotionRequest, Promotion>;
  readonly evaluationDataset: EndpointDef<{ id: string }, EvaluationDatasetDetail>;
  readonly createEvaluationDataset: EndpointDef<CreateEvaluationDatasetRequest, EvaluationDataset>;
  readonly createEvaluationCase: EndpointDef<CreateEvaluationCaseRequest, EvaluationCase>;
  readonly experiment: EndpointDef<{ id: string }, ExperimentDetail>;
  readonly createExperiment: EndpointDef<CreateExperimentRequest, Experiment>;
  readonly createExperimentResult: EndpointDef<CreateExperimentResultRequest, ExperimentResult>;
  readonly compileContext: EndpointDef<ContextCompileRequest, ContextSnapshot>;
  readonly contextSnapshot: EndpointDef<{ hash: string }, ContextSnapshot>;
} = {
  health: {
    method: 'GET',
    path: '/health',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as HealthResponse,
  },
  authVerify: {
    method: 'GET',
    path: '/auth/verify',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as AuthVerifyResponse,
  },
  vaultStatus: {
    method: 'GET',
    path: '/vault/status',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as VaultStatus,
  },
  vaultIndex: {
    method: 'POST',
    path: '/vault/index',
    auth: true,
    request: undefined as IndexOptions | undefined,
    response: undefined as unknown as IndexResult,
  },
  search: {
    method: 'GET',
    path: '/search',
    auth: false,
    request: undefined as unknown as SearchParams,
    response: undefined as unknown as SearchResult[],
  },
  noteById: {
    method: 'GET',
    path: '/notes/{id}',
    auth: false,
    request: undefined as unknown as { id: string },
    response: undefined as unknown as NoteDetail,
  },
  noteLinks: {
    method: 'GET',
    path: '/links/{id}',
    auth: false,
    request: undefined as unknown as { id: string },
    response: undefined as unknown as NoteLinks,
  },
  createNote: {
    method: 'POST',
    path: '/notes',
    auth: true,
    request: undefined as unknown as CreateNoteRequest,
    response: undefined as unknown as CreateNoteResponse,
  },
  updateNote: {
    method: 'PUT' as const,
    path: '/notes/{id}',
    auth: true,
    request: undefined as unknown as UpdateNoteRequest,
    response: undefined as unknown as UpdateNoteResponse,
  },
  deleteNote: {
    method: 'DELETE' as const,
    path: '/notes/{id}',
    auth: true,
    request: undefined as unknown as { id: string },
    response: undefined as unknown as DeleteNoteResponse,
  },
  capture: {
    method: 'POST',
    path: '/capture',
    auth: true,
    request: undefined as unknown as CaptureRequest,
    response: undefined as unknown as CaptureResponse,
  },
  ask: {
    method: 'POST',
    path: '/ask',
    auth: true,
    request: undefined as unknown as AskRequest,
    response: undefined as unknown as AskResponse,
  },
  projects: {
    method: 'GET',
    path: '/projects',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as Projects,
  },
  recent: {
    method: 'GET',
    path: '/recent',
    auth: false,
    request: undefined as unknown as RecentParams | undefined,
    response: undefined as unknown as SearchResult[],
  },
  stale: {
    method: 'GET',
    path: '/stale',
    auth: false,
    request: undefined as unknown as StaleParams | undefined,
    response: undefined as unknown as SearchResult[],
  },
  gitStatus: {
    method: 'GET',
    path: '/git/status',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as GitStatus,
  },
  graph: {
    method: 'GET',
    path: '/graph',
    auth: false,
    request: undefined as unknown as { center: string; depth?: number },
    response: undefined as unknown as Graph,
  },
  graphNeighbors: {
    method: 'GET',
    path: '/graph/neighbors',
    auth: false,
    request: undefined as unknown as { id: string },
    response: undefined as unknown as Graph,
  },
  promotions: {
    method: 'GET',
    path: '/promotions',
    auth: false,
    request: undefined as unknown as PromotionParams | undefined,
    response: undefined as unknown as Promotion[],
  },
  evaluationDataset: {
    method: 'GET',
    path: '/evaluation-datasets/{id}',
    auth: false,
    request: undefined as unknown as { id: string },
    response: undefined as unknown as EvaluationDatasetDetail,
  },
  experiment: {
    method: 'GET',
    path: '/experiments/{id}',
    auth: false,
    request: undefined as unknown as { id: string },
    response: undefined as unknown as ExperimentDetail,
  },
  proposePromotion: {
    method: 'POST',
    path: '/promotions',
    auth: true,
    request: undefined as unknown as ProposePromotionRequest,
    response: undefined as unknown as Promotion,
  },
  reviewPromotion: {
    method: 'POST',
    path: '/promotions/{id}/review',
    auth: true,
    request: undefined as unknown as ReviewPromotionRequest,
    response: undefined as unknown as Promotion,
  },
  commitPromotion: {
    method: 'POST',
    path: '/promotions/{id}/commit',
    auth: true,
    request: undefined as unknown as CommitPromotionRequest,
    response: undefined as unknown as Promotion,
  },
  createEvaluationDataset: {
    method: 'POST',
    path: '/evaluation-datasets',
    auth: true,
    request: undefined as unknown as CreateEvaluationDatasetRequest,
    response: undefined as unknown as EvaluationDataset,
  },
  createEvaluationCase: {
    method: 'POST',
    path: '/evaluation-datasets/{id}/cases',
    auth: true,
    request: undefined as unknown as CreateEvaluationCaseRequest,
    response: undefined as unknown as EvaluationCase,
  },
  createExperiment: {
    method: 'POST',
    path: '/experiments',
    auth: true,
    request: undefined as unknown as CreateExperimentRequest,
    response: undefined as unknown as Experiment,
  },
  createExperimentResult: {
    method: 'POST',
    path: '/experiments/{id}/results',
    auth: true,
    request: undefined as unknown as CreateExperimentResultRequest,
    response: undefined as unknown as ExperimentResult,
  },
  compileContext: {
    method: 'POST',
    path: '/agents/{id}/context',
    auth: true,
    request: undefined as unknown as ContextCompileRequest,
    response: undefined as unknown as ContextSnapshot,
  },
  contextSnapshot: {
    method: 'GET',
    path: '/contexts/{hash}',
    auth: false,
    request: undefined as unknown as { hash: string },
    response: undefined as unknown as ContextSnapshot,
  },
};

// Endpoint is preserved for internal use; callers typically index
// into `routes` directly by key.
export type Endpoint<
  M extends Method,
  P extends string,
> = (typeof routes)[KeyOfRoutesWith<M, P>];

type KeyOfRoutesWith<M extends Method, P extends string> = {
  [K in keyof typeof routes]: (typeof routes)[K] extends { method: M; path: P }
    ? K
    : never;
}[keyof typeof routes];
