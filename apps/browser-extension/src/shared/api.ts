// Thin wrapper over @agentvault/contract's HTTP client. The extension
// stores the auth token and base URL in chrome.storage.local; the contract
// package stays portable by accepting configuration from the caller.

import {
  createClient,
  type ApiClient,
  type AskRequest,
  type AskResponse,
  type AuthVerifyResponse,
  type CaptureRequest,
  type CaptureResponse,
  type CreateNoteRequest,
  type CreateNoteResponse,
  type GitStatus,
  type HealthResponse,
  type IndexOptions,
  type IndexResult,
  type NoteDetail,
  type RecentParams,
  type SearchParams,
  type SearchResult,
  type StaleParams,
  type VaultStatus,
  DEFAULT_BASE_URL,
} from '@agentvault/contract';
import type { CapturePayload } from './local';

const TOKEN_KEY = 'agentvault_token';
const BASE_URL_KEY = 'agentvault_base_url';

// Backward-compatible default exported for consumers that have not switched
// to the dynamic getBaseUrl() helper yet.
export { DEFAULT_BASE_URL as API_BASE };

// chromeStorageTokenStore is a TokenStore backed by chrome.storage.local.
// get() returns the in-memory cached value. The cache is refreshed by
// refreshToken() (on API calls) and by set/clear (on UI writes). The
// constructor pre-load was removed to avoid a race: without it, a popup
// write can never be clobbered by a late-firing init callback.
function chromeStorageTokenStore(): {
  get(): string | null;
  set(value: string): void;
  clear(): void;
} {
  let cached: string | null = null;
  return {
    get: () => cached,
    set: (v: string) => {
      cached = v;
      try {
        chrome.storage.local.set({ [TOKEN_KEY]: v });
      } catch {
        // ignore
      }
    },
    clear: () => {
      cached = '';
      try {
        chrome.storage.local.remove(TOKEN_KEY);
      } catch {
        // ignore
      }
    },
  };
}

const client: ApiClient = createClient({
  baseUrl: DEFAULT_BASE_URL,
  tokenStore: chromeStorageTokenStore(),
});

// Ensure the running client reflects the latest stored configuration on
// startup. Components that need to wait for this can import `apiReady`.
async function loadConfig(): Promise<void> {
  await Promise.all([refreshToken(), refreshBaseUrl()]);
}
export const apiReady = loadConfig();

// refreshToken re-reads the token from chrome.storage.local. Call this
// after setting the token from the popup so the cached value is up to
// date before the next HTTP request.
export async function refreshToken(): Promise<void> {
  const token = await getToken();
  client.setToken(token);
}

// refreshBaseUrl re-reads the base URL from chrome.storage.local. Call this
// before a request when the URL may have changed (e.g. after settings edit).
export async function refreshBaseUrl(): Promise<void> {
  const url = await getStoredBaseUrl();
  client.setBaseUrl(url);
}

export async function syncClientConfig(): Promise<void> {
  await refreshToken();
  await refreshBaseUrl();
}

// getToken reads the saved auth token from extension storage.
export async function getToken(): Promise<string> {
  const result = await new Promise<Record<string, unknown>>((resolve) => {
    try {
      chrome.storage.local.get(TOKEN_KEY, (r) => resolve(r || {}));
    } catch {
      resolve({});
    }
  });
  return (result?.[TOKEN_KEY] as string) || '';
}

export async function setToken(token: string): Promise<void> {
  await new Promise<void>((resolve) => {
    try {
      chrome.storage.local.set({ [TOKEN_KEY]: token }, () => resolve());
    } catch {
      resolve();
    }
  });
  client.setToken(token);
}

async function getStoredBaseUrl(): Promise<string> {
  const result = await new Promise<Record<string, unknown>>((resolve) => {
    try {
      chrome.storage.local.get(BASE_URL_KEY, (r) => resolve(r || {}));
    } catch {
      resolve({});
    }
  });
  const value = (result?.[BASE_URL_KEY] as string) || '';
  return value.trim() || DEFAULT_BASE_URL;
}

async function setStoredBaseUrl(url: string): Promise<void> {
  const normalized = url.trim() || DEFAULT_BASE_URL;
  await new Promise<void>((resolve) => {
    try {
      chrome.storage.local.set({ [BASE_URL_KEY]: normalized }, () => resolve());
    } catch {
      resolve();
    }
  });
}

function originFromUrl(url: string): string | null {
  try {
    const { protocol, host } = new URL(url);
    if (!protocol.startsWith('http')) return null;
    return `${protocol}//${host}`;
  } catch {
    return null;
  }
}

async function requestHostPermission(url: string): Promise<void> {
  const origin = originFromUrl(url);
  if (!origin || typeof chrome.permissions === 'undefined') return;
  try {
    await chrome.permissions.request({ origins: [`${origin}/*`] });
  } catch {
    // Ignore: permission may already be granted or the API may be unavailable.
  }
}

export async function getBaseUrl(): Promise<string> {
  await refreshBaseUrl();
  return client.getBaseUrl();
}

export async function setBaseUrl(url: string): Promise<void> {
  const normalized = (url.trim() || DEFAULT_BASE_URL).replace(/\/$/, '');
  await Promise.all([setStoredBaseUrl(normalized), requestHostPermission(normalized)]);
  client.setBaseUrl(normalized);
}

export async function checkHealth(): Promise<boolean> {
  await syncClientConfig();
  try {
    await client.checkHealth();
    return true;
  } catch {
    return false;
  }
}

export async function checkAuth(): Promise<AuthVerifyResponse | null> {
  await syncClientConfig();
  try {
    return await client.verifyAuth();
  } catch {
    return null;
  }
}

export async function getVaultStatus(): Promise<VaultStatus | null> {
  await syncClientConfig();
  try {
    return await client.getVaultStatus();
  } catch {
    return null;
  }
}

export async function triggerIndex(opts?: IndexOptions): Promise<IndexResult | null> {
  await syncClientConfig();
  try {
    return await client.triggerIndex(opts);
  } catch {
    return null;
  }
}

export async function getNote(id: string): Promise<NoteDetail | null> {
  await syncClientConfig();
  try {
    return await client.getNote(id);
  } catch {
    return null;
  }
}

// Alias used by sibling popup components.
export const getNoteDetail = getNote;

export async function createNote(req: CreateNoteRequest): Promise<CreateNoteResponse> {
  await syncClientConfig();
  return client.createNote(req);
}

export async function ask(req: AskRequest): Promise<AskResponse> {
  await syncClientConfig();
  return client.ask(req);
}

// Convenience alias used by the Ask popup component.
export async function askVault(question: string): Promise<AskResponse> {
  return ask({ question });
}

// sendCapture keeps its old shape (returning `{ path: string }`) and
// accepts the extension's `CapturePayload` (which has extra client-only
// fields). It strips the client-only fields before sending.
export async function sendCapture(payload: CapturePayload): Promise<CaptureResponse> {
  await syncClientConfig();
  const body: CaptureRequest = {
    type: payload.type,
    title: payload.title,
    url: payload.url,
    text: payload.text ?? payload.selectedText,
    project: payload.project,
    tags: payload.tags,
  };
  return client.capture(body);
}

export async function searchVault(queryOrParams: string | SearchParams): Promise<SearchResult[]> {
  await syncClientConfig();
  const params: SearchParams = typeof queryOrParams === 'string'
    ? { q: queryOrParams }
    : queryOrParams;
  if (!params.q?.trim()) return [];
  return client.search(params);
}

export async function getProjects(): Promise<string[]> {
  await syncClientConfig();
  try {
    return await client.getProjects();
  } catch {
    return [];
  }
}

export async function getRecent(params?: RecentParams): Promise<SearchResult[]> {
  await syncClientConfig();
  try {
    return await client.getRecent(params);
  } catch {
    return [];
  }
}

// Alias used by the Recent popup component.
export async function getRecentNotes(params?: RecentParams): Promise<SearchResult[]> {
  return getRecent(params);
}

export async function getStale(params?: StaleParams): Promise<SearchResult[]> {
  await syncClientConfig();
  try {
    return await client.getStale(params);
  } catch {
    return [];
  }
}

export async function getGitStatus(): Promise<GitStatus | null> {
  await syncClientConfig();
  try {
    return await client.getGitStatus();
  } catch {
    return null;
  }
}
