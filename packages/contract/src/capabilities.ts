export type MutationCapability =
  | 'mutation:read'
  | 'mutation:propose'
  | 'mutation:approve'
  | 'mutation:commit'
  | 'mutation:undo'
  | 'mutation:reject';

export interface CapabilityScope {
  pathPrefixes?: string[];
  projects?: string[];
  sessions?: string[];
}

export interface CapabilityPrincipal {
  id: string;
  agentId: string;
  capabilities: MutationCapability[];
  scope?: CapabilityScope;
  createdAt: string;
  expiresAt?: string;
  revokedAt?: string;
}

export interface MintCapabilityRequest {
  id?: string;
  agentId: string;
  capabilities: MutationCapability[];
  scope?: CapabilityScope;
  expiresAt?: string;
}

/** Raw tokens are returned only at issuance and are never persisted in plaintext. */
export interface IssuedCapabilityToken {
  principal: CapabilityPrincipal;
  token: string;
}
