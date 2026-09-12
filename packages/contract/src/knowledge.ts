// Universal knowledge and durable agent-state contract. These types mirror
// core/internal/contract/knowledge.go and are UI-independent so other
// AgentVault-integrated products can consume the same stable API surface.

export interface ProvenanceEvidence {
  source: string;
  id?: string;
  path?: string;
  quote?: string;
}

export interface ProvenanceRecord {
  id: string;
  sourceType: string;
  sourceId?: string;
  agentId?: string;
  sessionId?: string;
  model?: string;
  confidence: number;
  observedAt: string;
  evidence?: ProvenanceEvidence[];
  metadata?: Record<string, unknown>;
  createdAt: string;
}

export type CreateProvenanceRequest = Omit<
  ProvenanceRecord,
  'id' | 'createdAt' | 'observedAt' | 'confidence'
> & {
  id?: string;
  confidence?: number;
  observedAt?: string;
};

export interface KnowledgeObject {
  id: string;
  type: string;
  title: string;
  status?: string;
  organization?: string;
  project?: string;
  canonicalPath?: string;
  data?: Record<string, unknown>;
  provenanceId?: string;
  createdAt: string;
  updatedAt: string;
}

export interface UpsertKnowledgeObjectRequest {
  id?: string;
  type: string;
  title: string;
  status?: string;
  organization?: string;
  project?: string;
  canonicalPath?: string;
  data?: Record<string, unknown>;
  provenanceId?: string;
}

export interface KnowledgeObjectFilter {
  type?: string;
  organization?: string;
  project?: string;
  status?: string;
  limit?: number;
}

export interface ObjectRelation {
  id: string;
  fromObjectId: string;
  toObjectId: string;
  relationType: string;
  validFrom?: string;
  validTo?: string;
  confidence: number;
  provenanceId?: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface CreateObjectRelationRequest {
  id?: string;
  fromObjectId: string;
  toObjectId: string;
  relationType: string;
  validFrom?: string;
  validTo?: string;
  confidence?: number;
  provenanceId?: string;
  metadata?: Record<string, unknown>;
}

export type MemoryClass = 'working' | 'episodic' | 'semantic' | 'procedural';

export type MemoryKind =
  | 'observation'
  | 'episode'
  | 'fact'
  | 'preference'
  | 'decision'
  | 'procedure'
  | 'constraint'
  | 'summary';

export interface MemoryRecord {
  id: string;
  memoryClass: MemoryClass;
  memoryKind?: MemoryKind;
  scopeType: string;
  scopeId: string;
  content: string;
  objectId?: string;
  provenanceId?: string;
  confidence: number;
  validFrom?: string;
  validTo?: string;
  supersedesId?: string;
  metadata?: Record<string, unknown>;
  createdAt: string;
  updatedAt: string;
}

export interface CreateMemoryRequest {
  id?: string;
  memoryClass: MemoryClass;
  memoryKind?: MemoryKind;
  scopeType: string;
  scopeId: string;
  content: string;
  objectId?: string;
  provenanceId?: string;
  confidence?: number;
  validFrom?: string;
  validTo?: string;
  supersedesId?: string;
  metadata?: Record<string, unknown>;
}

export interface MemoryFilter {
  scopeType: string;
  scopeId: string;
  memoryClass?: MemoryClass;
  memoryKind?: MemoryKind;
  limit?: number;
}

export interface SessionEvent {
  id: string;
  sessionId: string;
  eventType: string;
  payload?: Record<string, unknown>;
  provenanceId?: string;
  createdAt: string;
}

export interface AgentSession {
  id: string;
  agentId: string;
  project?: string;
  objective: string;
  status: string;
  branch?: string;
  worktree?: string;
  context?: Record<string, unknown>;
  startedAt: string;
  updatedAt: string;
  endedAt?: string;
  events?: SessionEvent[];
}

export interface StartAgentSessionRequest {
  id?: string;
  agentId: string;
  project?: string;
  objective: string;
  branch?: string;
  worktree?: string;
  context?: Record<string, unknown>;
}

export interface AppendSessionEventRequest {
  id?: string;
  eventType: string;
  payload?: Record<string, unknown>;
  provenanceId?: string;
}

export interface CloseAgentSessionRequest {
  status?: string;
}
