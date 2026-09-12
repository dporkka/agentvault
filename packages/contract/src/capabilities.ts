export type MutationCapability =
  | 'mutation:read'
  | 'mutation:propose'
  | 'mutation:approve'
  | 'mutation:commit'
  | 'mutation:undo'
  | 'mutation:reject';

export type ReadCapability =
  | 'vault:read'
  | 'knowledge:read'
  | 'context:compile'
  | 'ai:invoke';

export type Capability = MutationCapability | ReadCapability;

export interface CapabilityScope {
  pathPrefixes?: string[];
  projects?: string[];
  sessions?: string[];
}

export interface CapabilityPrincipal {
  id: string;
  agentId: string;
  capabilities: Capability[];
  scope?: CapabilityScope;
  createdAt: string;
  expiresAt?: string;
  revokedAt?: string;
}

export interface MintCapabilityRequest {
  id?: string;
  agentId: string;
  capabilities: Capability[];
  scope?: CapabilityScope;
  expiresAt?: string;
}

/** Raw tokens are returned only at issuance and are never persisted in plaintext. */
export interface IssuedCapabilityToken {
  principal: CapabilityPrincipal;
  token: string;
}
