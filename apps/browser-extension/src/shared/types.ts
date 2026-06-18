// Shared types for the browser extension. Server-facing shapes come
// from @agentvault/contract; the popup, content script, and background
// service worker share those types directly. Local UI/messaging types
// (CapturePayload, PageData) are re-exported from `./local`.

export type { SearchResult } from '@agentvault/contract';
export { ApiError, type ApiClient, DEFAULT_BASE_URL } from '@agentvault/contract';

export type { CapturePayload, PageData } from './local';
