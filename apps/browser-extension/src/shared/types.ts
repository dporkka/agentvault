// Shared types for the browser extension. Server-facing shapes come
// from @agentvault/contract; the popup, content script, and background
// service worker share those types directly. Local UI/messaging types
// (CapturePayload, PageData) are re-exported from `./local`.

export type {
  AskRequest,
  AskResponse,
  CreateNoteRequest,
  CreateNoteResponse,
  GitStatus,
  HealthResponse,
  IndexOptions,
  IndexResult,
  NoteDetail,
  RecentParams,
  SearchParams,
  SearchResult,
  StaleParams,
  VaultStatus,
} from '@agentvault/contract';
export { ApiError, type ApiClient, DEFAULT_BASE_URL } from '@agentvault/contract';

export type { CapturePayload, PageData } from './local';
export { classifyError, errorKindLabel, errorRecoveryHint } from './errors';
export type { ErrorKind, ClassifiedError } from './errors';
export { sendOrQueueCapture, retryQueuedCaptures, getPendingCount, removeQueuedCapture, listQueuedCaptures } from './capture-queue';
export type { CaptureSyncState, QueuedCapture, CaptureResult } from './capture-queue';
