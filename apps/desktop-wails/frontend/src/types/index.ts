export interface VaultStatus {
  path: string;
  isOpen: boolean;
  noteCount: number;
  version: string;
}

export interface Note {
  id: string;
  title: string;
  path: string;
  type: string;
  project: string;
  status: string;
  tags: string[];
  body: string;
  updatedAt: string;
}

export interface SearchResult {
  id: string;
  title: string;
  path: string;
  type: string;
  project: string;
  status?: string;
  tags: string[];
  snippet: string;
  updatedAt: string;
}

export interface IndexingStatus {
  isIndexing: boolean;
  noteCount: number;
}

export interface Answer {
  answer: string;
  sources: Source[];
  confidence: string;
  caveats: string[];
  missingInfo: string;
  suggestedActions: string[];
}

export interface Source {
  path: string;
  title: string;
  excerpt: string;
}

export type ViewName =
  | 'editor'
  | 'search'
  | 'projects'
  | 'decisions'
  | 'settings';
