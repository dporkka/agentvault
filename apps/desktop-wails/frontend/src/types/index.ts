// Canonical data types for the desktop frontend. Server-facing types
// (SearchResult, Answer, Source) come from @agentvault/contract, which
// mirrors the Go HTTP API shapes. Wails-specific shapes (VaultStatus,
// Note, IndexingStatus) are the Go service shapes shared with the HTTP
// contract via core/internal/contract.

import type {
  AskResponse,
  AskSource,
  NoteDetail,
  SearchResult,
  VaultStatus,
} from '@agentvault/contract';

export type { SearchResult, VaultStatus };
export type Answer = AskResponse;
export type Source = AskSource;

// Wails-only types that have no HTTP equivalent. These mirror the
// auto-generated Wails bridge types (the wails generate module output)
// so the frontend and the Go struct stay aligned. Note is aliased
// from the HTTP contract's NoteDetail.
export type Note = NoteDetail;

export interface IndexingStatus {
  isIndexing: boolean;
  noteCount: number;
}

export type ViewName =
  | 'editor'
  | 'search'
  | 'projects'
  | 'decisions'
  | 'settings';
