import {
  ApiError,
  DEFAULT_BASE_URL,
  inMemoryTokenStore,
  type TokenStore,
} from './client';
import type {
  CapabilityPrincipal,
  IssuedCapabilityToken,
  MintCapabilityRequest,
} from './capabilities';
import type {
  CompileContextRequest,
  ContextBundle,
} from './context';
import type {
  AgentSession,
  AppendSessionEventRequest,
  CloseAgentSessionRequest,
  CreateMemoryRequest,
  CreateObjectRelationRequest,
  CreateProvenanceRequest,
  KnowledgeObject,
  KnowledgeObjectFilter,
  MemoryFilter,
  MemoryRecord,
  ObjectRelation,
  ProvenanceRecord,
  SessionEvent,
  StartAgentSessionRequest,
  UpsertKnowledgeObjectRequest,
} from './knowledge';
import type {
  ApproveMutationRequest,
  CreateMutationProposalRequest,
  MutationProposal,
  MutationProposalFilter,
  MutationResult,
  RejectMutationRequest,
} from './mutations';

export interface KnowledgeClientOptions {
  baseUrl?: string;
  token?: string;
  fetchImpl?: typeof fetch;
  tokenStore?: TokenStore;
}

export interface KnowledgeClient {
  compileContext(req: CompileContextRequest): Promise<ContextBundle>;
  listObjects(filter?: KnowledgeObjectFilter): Promise<KnowledgeObject[]>;
  getObject(id: string): Promise<KnowledgeObject>;
  upsertObject(req: UpsertKnowledgeObjectRequest): Promise<KnowledgeObject>;
  updateObject(id: string, req: UpsertKnowledgeObjectRequest): Promise<KnowledgeObject>;
  createRelation(req: CreateObjectRelationRequest): Promise<ObjectRelation>;
  getObjectRelations(id: string): Promise<ObjectRelation[]>;
  createProvenance(req: CreateProvenanceRequest): Promise<ProvenanceRecord>;
  getProvenance(id: string): Promise<ProvenanceRecord>;
  recordMemory(req: CreateMemoryRequest): Promise<MemoryRecord>;
  listMemories(filter: MemoryFilter): Promise<MemoryRecord[]>;
  startSession(req: StartAgentSessionRequest): Promise<AgentSession>;
  getSession(id: string): Promise<AgentSession>;
  appendSessionEvent(id: string, req: AppendSessionEventRequest): Promise<SessionEvent>;
  closeSession(id: string, req?: CloseAgentSessionRequest): Promise<AgentSession>;

  /** Dry-run and persist a reviewable mutation proposal. Never changes the target file. */
  proposeMutation(req: CreateMutationProposalRequest): Promise<MutationProposal>;
  listMutations(filter?: MutationProposalFilter): Promise<MutationProposal[]>;
  getMutation(id: string): Promise<MutationProposal>;
  /** Applies an explicit mutation approval when the current token carries mutation:approve. */
  approveMutation(id: string, req?: ApproveMutationRequest): Promise<MutationProposal>;
  /** Applies an approved proposal when the current token carries mutation:commit. */
  commitMutation(id: string): Promise<MutationResult>;
  /** Restores the captured before-state when the current token carries mutation:undo. */
  undoMutation(id: string): Promise<MutationResult>;
  rejectMutation(id: string, req?: RejectMutationRequest): Promise<MutationProposal>;

  /** Root-token-only administration. Raw capability tokens are returned only by mintCapability. */
  listCapabilities(): Promise<CapabilityPrincipal[]>;
  mintCapability(req: MintCapabilityRequest): Promise<IssuedCapabilityToken>;
  revokeCapability(id: string): Promise<CapabilityPrincipal>;
}

function queryString<T extends object>(values: T): string {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values)) {
    if (value === undefined || value === null || value === '') continue;
    params.set(key, String(value));
  }
  return params.toString();
}

export function createKnowledgeClient(opts: KnowledgeClientOptions = {}): KnowledgeClient {
  const fetchImpl = opts.fetchImpl ?? (typeof fetch !== 'undefined' ? fetch : (() => {
    throw new Error('No fetch implementation available; pass fetchImpl to createKnowledgeClient.');
  }) as unknown as typeof fetch);
  const baseUrl = (opts.baseUrl ?? DEFAULT_BASE_URL).replace(/\/$/, '');
  const tokenStore = opts.tokenStore ?? inMemoryTokenStore(opts.token ?? '');

  async function call<T>(method: 'GET' | 'POST' | 'PUT', path: string, body?: unknown): Promise<T> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    const token = tokenStore.get();
    if (token) headers['X-AgentVault-Token'] = token;

    let response: Response;
    try {
      response = await fetchImpl(`${baseUrl}${path}`, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch {
      throw new ApiError(`Cannot connect to AgentVault server at ${baseUrl}. Is the server running?`, 0);
    }

    if (!response.ok) {
      let message = `HTTP ${response.status}`;
      try {
        const payload = await response.json() as { error?: string };
        if (payload.error) message = payload.error;
      } catch {
        // Preserve the HTTP status fallback.
      }
      throw new ApiError(message, response.status);
    }
    return await response.json() as T;
  }

  return {
    compileContext(req) {
      return call<ContextBundle>('POST', '/context/compile', req);
    },
    listObjects(filter = {}) {
      const qs = queryString(filter);
      return call<KnowledgeObject[]>('GET', qs ? `/objects?${qs}` : '/objects');
    },
    getObject(id) {
      return call<KnowledgeObject>('GET', `/objects/${encodeURIComponent(id)}`);
    },
    upsertObject(req) {
      return call<KnowledgeObject>('POST', '/objects', req);
    },
    updateObject(id, req) {
      return call<KnowledgeObject>('PUT', `/objects/${encodeURIComponent(id)}`, req);
    },
    createRelation(req) {
      return call<ObjectRelation>('POST', '/relations', req);
    },
    getObjectRelations(id) {
      return call<ObjectRelation[]>('GET', `/objects/${encodeURIComponent(id)}/relations`);
    },
    createProvenance(req) {
      return call<ProvenanceRecord>('POST', '/provenance', req);
    },
    getProvenance(id) {
      return call<ProvenanceRecord>('GET', `/provenance/${encodeURIComponent(id)}`);
    },
    recordMemory(req) {
      return call<MemoryRecord>('POST', '/memory', req);
    },
    listMemories(filter) {
      const qs = queryString(filter);
      return call<MemoryRecord[]>('GET', `/memory?${qs}`);
    },
    startSession(req) {
      return call<AgentSession>('POST', '/sessions', req);
    },
    getSession(id) {
      return call<AgentSession>('GET', `/sessions/${encodeURIComponent(id)}`);
    },
    appendSessionEvent(id, req) {
      return call<SessionEvent>('POST', `/sessions/${encodeURIComponent(id)}/events`, req);
    },
    closeSession(id, req = {}) {
      return call<AgentSession>('POST', `/sessions/${encodeURIComponent(id)}/close`, req);
    },
    proposeMutation(req) {
      return call<MutationProposal>('POST', '/mutations', req);
    },
    listMutations(filter = {}) {
      const qs = queryString(filter);
      return call<MutationProposal[]>('GET', qs ? `/mutations?${qs}` : '/mutations');
    },
    getMutation(id) {
      return call<MutationProposal>('GET', `/mutations/${encodeURIComponent(id)}`);
    },
    approveMutation(id, req = { approvedBy: '' }) {
      return call<MutationProposal>('POST', `/mutations/${encodeURIComponent(id)}/approve`, req);
    },
    commitMutation(id) {
      return call<MutationResult>('POST', `/mutations/${encodeURIComponent(id)}/commit`);
    },
    undoMutation(id) {
      return call<MutationResult>('POST', `/mutations/${encodeURIComponent(id)}/undo`);
    },
    rejectMutation(id, req = { actor: '', reason: '' }) {
      return call<MutationProposal>('POST', `/mutations/${encodeURIComponent(id)}/reject`, req);
    },
    listCapabilities() {
      return call<CapabilityPrincipal[]>('GET', '/auth/capabilities');
    },
    mintCapability(req) {
      return call<IssuedCapabilityToken>('POST', '/auth/capabilities', req);
    },
    revokeCapability(id) {
      return call<CapabilityPrincipal>('POST', `/auth/capabilities/${encodeURIComponent(id)}/revoke`);
    },
  };
}
