// Thin re-export of the shared @agentvault/contract client. The web app
// instantiates one client backed by localStorage and keeps the same
// `api` named export so the rest of the components don't need to change.

import {
  ApiError,
  createClient,
  getDefaultClient,
  type ApiClient,
  type CaptureRequest,
  type CaptureResponse,
  type CreateNoteRequest,
  type CreateNoteResponse,
  type HealthResponse,
  type IndexResult,
  type NoteDetail,
  type SearchResult,
  type VaultStatus,
} from '@agentvault/contract';

// A single shared client instance. Components import `api` and call its
// methods. Underlying base URL and token are persisted to localStorage
// via the contract package's default stores.
export const api: ApiClient = (() => {
  if (typeof window === 'undefined') {
    return createClient();
  }
  return getDefaultClient();
})();

export { ApiError };
export type {
  CaptureRequest,
  CaptureResponse,
  CreateNoteRequest,
  CreateNoteResponse,
  HealthResponse,
  IndexResult,
  NoteDetail,
  SearchResult,
  VaultStatus,
};
