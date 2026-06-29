// Zero-dependency typed HTTP client for the AgentVault local server.
// Apps instantiate it via `createClient(...)` so they can plug in their
// own token storage (localStorage for web, chrome.storage for the
// extension, AsyncStorage for mobile). The package itself never touches
// `localStorage` or `globalThis` so it works in any environment
// (browser, content script, service worker, RN/Metro bundle, SSR).

import type {
  AskRequest,
  AskResponse,
  AuthVerifyResponse,
  CaptureRequest,
  CaptureResponse,
  CreateNoteRequest,
  CreateNoteResponse,
  GitStatus,
  HealthResponse,
  IndexOptions,
  IndexResult,
  NoteDetail,
  Projects,
  RecentParams,
  SearchParams,
  SearchResult,
  StaleParams,
  VaultStatus,
} from './types';

export const DEFAULT_BASE_URL = 'http://127.0.0.1:47321';

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

export interface TokenStore {
  get(): string | null;
  set(value: string): void;
  clear(): void;
}

// inMemoryTokenStore is the default token store used when an app does not
// pass its own. Suitable for read-only clients and tests.
export function inMemoryTokenStore(initial = ''): TokenStore {
  let value = initial;
  return {
    get: () => value,
    set: (v: string) => {
      value = v;
    },
    clear: () => {
      value = '';
    },
  };
}

export interface CreateClientOptions {
  baseUrl?: string;
  token?: string;
  fetchImpl?: typeof fetch;
  tokenStore?: TokenStore;
}

export interface ApiClient {
  setBaseUrl(url: string): void;
  setToken(token: string): void;
  getBaseUrl(): string;
  getToken(): string;
  checkHealth(): Promise<HealthResponse>;
  verifyAuth(): Promise<AuthVerifyResponse>;
  getVaultStatus(): Promise<VaultStatus>;
  triggerIndex(opts?: IndexOptions): Promise<IndexResult>;
  search(params: SearchParams): Promise<SearchResult[]>;
  getNote(id: string): Promise<NoteDetail>;
  createNote(req: CreateNoteRequest): Promise<CreateNoteResponse>;
  capture(req: CaptureRequest): Promise<CaptureResponse>;
  ask(req: AskRequest): Promise<AskResponse>;
  getProjects(): Promise<Projects>;
  getRecent(params?: RecentParams): Promise<SearchResult[]>;
  getStale(params?: StaleParams): Promise<SearchResult[]>;
  getGitStatus(): Promise<GitStatus>;
}

function buildSearch(params: SearchParams | RecentParams | StaleParams | undefined): string {
  const sp = new URLSearchParams();
  if (!params) return '';
  // The server expects snake_case for these query params while the TS API
  // stays camelCase to match the rest of the contract.
  const keyMap: Record<string, string> = {
    hybridWeight: 'hybrid_weight',
  };
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === null) continue;
    const key = keyMap[k] ?? k;
    if (typeof v === 'boolean') {
      if (v) sp.set(key, 'true');
    } else {
      sp.set(key, String(v));
    }
  }
  return sp.toString();
}

export function createClient(opts: CreateClientOptions = {}): ApiClient {
  const fetchImpl = opts.fetchImpl ?? (typeof fetch !== 'undefined' ? fetch : (() => {
    throw new Error('No fetch implementation available; pass fetchImpl to createClient.');
  }) as unknown as typeof fetch);
  let baseUrl = (opts.baseUrl ?? DEFAULT_BASE_URL).replace(/\/$/, '');
  const tokenStore = opts.tokenStore ?? inMemoryTokenStore(opts.token ?? '');

  async function call<T>(method: 'GET' | 'POST', path: string, body?: unknown, auth = true): Promise<T> {
    const url = path.startsWith('http') ? path : `${baseUrl}${path}`;
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (auth) {
      const token = tokenStore.get();
      if (token) headers['X-AgentVault-Token'] = token;
    }
    let response: Response;
    try {
      response = await fetchImpl(url, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
      });
    } catch (err) {
      throw new ApiError(
        `Cannot connect to AgentVault server at ${baseUrl}. Is the server running?`,
        0
      );
    }
    if (!response.ok) {
      let message = `HTTP ${response.status}`;
      try {
        const text = await response.text();
        if (text) {
          try {
            const data = JSON.parse(text);
            if (data && typeof data === 'object' && 'error' in data) {
              message = (data as { error: string }).error || message;
            } else {
              message = text;
            }
          } catch {
            message = text;
          }
        }
      } catch { /* ignore */ }
      throw new ApiError(message, response.status);
    }
    if (response.status === 204) {
      return undefined as T;
    }
    try {
      return (await response.json()) as T;
    } catch (err) {
      throw new ApiError(
        `Invalid JSON response from ${url}: ${err instanceof Error ? err.message : String(err)}`,
        500
      );
    }
  }

  return {
    setBaseUrl(url: string) {
      baseUrl = url.replace(/\/$/, '');
    },
    setToken(token: string) {
      if (token) tokenStore.set(token);
      else tokenStore.clear();
    },
    getBaseUrl() {
      return baseUrl;
    },
    getToken() {
      return tokenStore.get() ?? '';
    },
    checkHealth() {
      return call<HealthResponse>('GET', '/health', undefined, false);
    },
    verifyAuth() {
      return call<AuthVerifyResponse>('GET', '/auth/verify', undefined, false);
    },
    getVaultStatus() {
      return call<VaultStatus>('GET', '/vault/status', undefined, false);
    },
    async triggerIndex(idxOpts?: IndexOptions) {
      return call<IndexResult>('POST', '/vault/index', idxOpts ?? {});
    },
    search(params) {
      const qs = buildSearch(params);
      return call<SearchResult[]>('GET', qs ? `/search?${qs}` : '/search', undefined, false);
    },
    getNote(id) {
      return call<NoteDetail>('GET', `/notes/${encodeURIComponent(id)}`, undefined, false);
    },
    createNote(req) {
      return call<CreateNoteResponse>('POST', '/notes', req);
    },
    capture(req) {
      return call<CaptureResponse>('POST', '/capture', req);
    },
    ask(req) {
      return call<AskResponse>('POST', '/ask', req);
    },
    getProjects() {
      return call<Projects>('GET', '/projects', undefined, false);
    },
    getRecent(params) {
      const qs = buildSearch(params);
      return call<SearchResult[]>('GET', qs ? `/recent?${qs}` : '/recent', undefined, false);
    },
    getStale(params) {
      const qs = buildSearch(params);
      return call<SearchResult[]>('GET', qs ? `/stale?${qs}` : '/stale', undefined, false);
    },
    getGitStatus() {
      return call<GitStatus>('GET', '/git/status', undefined, false);
    },
  };
}

// localStorageTokenStore is the helper web/web-extension apps use to
// back the tokenStore with `localStorage`. Only call this in a context
// where `localStorage` exists (do not import this on the server or in
// service workers that lack localStorage).
export function localStorageTokenStore(
  storage: Storage,
  key = 'agentvault_token',
): TokenStore {
  return {
    get: () => storage.getItem(key),
    set: (v: string) => {
      storage.setItem(key, v);
    },
    clear: () => {
      storage.removeItem(key);
    },
  };
}

export function localStorageBaseUrlStore(
  storage: Storage,
  key = 'agentvault_server_url',
): { get(): string; set(value: string): void } {
  return {
    get: () => storage.getItem(key) ?? DEFAULT_BASE_URL,
    set: (v: string) => {
      storage.setItem(key, v);
    },
  };
}

// getDefaultClient returns a fresh client each call backed by
// localStorage (when available). Creating a client is cheap.
export function getDefaultClient(): ApiClient {
  if (typeof localStorage === 'undefined') {
    return createClient();
  }
  const baseStore = localStorageBaseUrlStore(localStorage);
  const tokenStore = localStorageTokenStore(localStorage);
  return createClient({
    baseUrl: baseStore.get(),
    tokenStore,
  });
}
