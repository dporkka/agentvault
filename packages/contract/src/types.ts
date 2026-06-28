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

// POST /capture
export interface CaptureRequest {
  type?: string;
  title?: string;
  url?: string;
  text?: string;
  project?: string;
  tags?: string[];
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
