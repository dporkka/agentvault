export interface HealthResponse {
  status: string;
  vault: string;
  version: string;
}

export interface VaultStatus {
  path: string;
  isVault: boolean;
  noteCount: number;
  version: string;
}

export interface SearchResult {
  id: string;
  title: string;
  path: string;
  type: string;
  project: string;
  tags: string[];
  snippet: string;
  updatedAt: string;
}

export interface Note {
  id: string;
  title: string;
  path: string;
  type: string;
  content: string;
}

export interface Source {
  id: string;
  path: string;
  title: string;
  excerpt: string;
}

export interface AskResponse {
  answer: string;
  sources: Source[];
  confidence: string;
}

export interface CreateNoteData {
  type: string;
  title: string;
  project?: string;
  tags?: string[];
}

export interface CreateNoteResponse {
  path: string;
  id: string;
}
