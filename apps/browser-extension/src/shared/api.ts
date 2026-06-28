// Thin wrapper over @agentvault/contract's HTTP client. The extension
// stores the auth token in chrome.storage.local; the contract package
// stays portable by accepting a TokenStore from the caller.

import {
  createClient,
  type ApiClient,
  type AuthVerifyResponse,
  type CaptureRequest,
  type CaptureResponse,
  type SearchParams,
  type SearchResult,
  DEFAULT_BASE_URL,
} from '@agentvault/contract';
import type { CapturePayload } from './local';

const TOKEN_KEY = 'agentvault_token';

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

// refreshToken re-reads the token from chrome.storage.local. Call this
// after setting the token from the popup so the cached value is up to
// date before the next HTTP request.
export async function refreshToken(): Promise<void> {
  const token = await getToken();
  client.setToken(token);
}

// checkAuth returns the server's verifyAuth response, or null if the
// server is unreachable. Use this to show "token valid / missing / invalid"
// status in the popup without performing a write.
export async function checkAuth(): Promise<AuthVerifyResponse | null> {
  await refreshToken();
  try {
    return await client.verifyAuth();
  } catch {
    return null;
  }
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

export async function checkHealth(): Promise<boolean> {
  try {
    await client.checkHealth();
    return true;
  } catch {
    return false;
  }
}

// sendCapture keeps its old shape (returning `{ path: string }`) and
// accepts the extension's `CapturePayload` (which has extra client-only
// fields). It strips the client-only fields before sending.
export async function sendCapture(payload: CapturePayload): Promise<CaptureResponse> {
  await refreshToken();
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
  const params: SearchParams = typeof queryOrParams === 'string'
    ? { q: queryOrParams }
    : queryOrParams;
  if (!params.q?.trim()) return [];
  return client.search(params);
}

export async function getProjects(): Promise<string[]> {
  try {
    return await client.getProjects();
  } catch {
    return [];
  }
}
