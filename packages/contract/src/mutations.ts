export type MutationKind = 'create' | 'replace' | 'delete';

export type MutationStatus =
  | 'proposed'
  | 'approved'
  | 'committing'
  | 'committed'
  | 'undoing'
  | 'undone'
  | 'conflicted'
  | 'rejected';

export interface CreateMutationProposalRequest {
  kind: MutationKind;
  path: string;
  /** Required for create/replace, including an explicit empty string; omit for delete. */
  content?: string;
  reason: string;
  agentId?: string;
  sessionId?: string;
  provenanceId?: string;
}

export interface ApproveMutationRequest {
  /** Audit label supplied by the trusted control-plane caller; not a cryptographic identity claim. */
  approvedBy: string;
}

export interface RejectMutationRequest {
  /** Audit label supplied by the trusted control-plane caller. */
  actor: string;
  reason?: string;
}

export interface MutationProposal {
  id: string;
  kind: MutationKind;
  path: string;
  reason: string;
  agentId?: string;
  sessionId?: string;
  provenanceId?: string;
  status: MutationStatus;
  beforeExists: boolean;
  afterExists: boolean;
  beforeHash?: string;
  afterHash?: string;
  beforeContent?: string;
  afterContent?: string;
  diff: string;
  approvedBy?: string;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
  approvedAt?: string;
  committedAt?: string;
  undoneAt?: string;
}

export interface MutationResult {
  proposal: MutationProposal;
  warnings?: string[];
}

export interface MutationProposalFilter {
  status?: MutationStatus;
  agentId?: string;
  sessionId?: string;
  limit?: number;
}
