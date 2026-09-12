import type { ProvenanceEvidence } from './knowledge';

export interface CompileContextRequest {
  task: string;
  project?: string;
  agentId?: string;
  sessionId?: string;
  objectIds?: string[];
  tokenBudget?: number;
  maxItems?: number;
  asOf?: string;
}

export interface ContextProvenance {
  id: string;
  sourceType: string;
  sourceId?: string;
  agentId?: string;
  sessionId?: string;
  model?: string;
  confidence: number;
  observedAt: string;
  evidence?: ProvenanceEvidence[];
}

export interface ContextItem {
  kind: 'session' | 'session_event' | 'session_history' | 'memory' | 'object' | 'relation' | 'note' | string;
  id: string;
  title?: string;
  content: string;
  path?: string;
  score: number;
  estimatedTokens: number;
  objectIds?: string[];
  provenance?: ContextProvenance;
  metadata?: Record<string, unknown>;
}

export interface ContextBundleStats {
  candidates: number;
  included: number;
  dropped: number;
  byKind: Record<string, number>;
}

export interface ContextBundle {
  version: string;
  task: string;
  project?: string;
  agentId?: string;
  sessionId?: string;
  asOf: string;
  tokenBudget: number;
  estimatedTokens: number;
  truncated: boolean;
  items: ContextItem[];
  stats: ContextBundleStats;
}
