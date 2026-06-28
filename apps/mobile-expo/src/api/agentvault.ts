// Thin wrapper over @agentvault/contract's HTTP client. The mobile app
// reads the base URL and auth token from AsyncStorage (via the existing
// localInbox settings store); the contract package stays portable by
// accepting a TokenStore from the caller.

import {
  createClient,
  type ApiClient,
  type AuthVerifyResponse,
  type SearchResult,
  DEFAULT_BASE_URL,
} from '@agentvault/contract';
import { getSettings } from '../storage/localInbox';
import type { Capture } from '../types';

export { DEFAULT_BASE_URL };

// asyncTokenStore keeps the token in memory once loaded and is the
// minimal shim needed by the contract client. The actual source of truth
// is AsyncStorage; this layer just caches it for the client to read
// synchronously. The mobile app never re-saves the token from the
// contract client side (the Settings screen writes through
// `localInbox.saveSettings`), so set/clear are no-ops.
function asyncTokenStore() {
  let cached: string | null = null;
  // Pre-load asynchronously.
  getSettings()
    .then((s) => {
      cached = s.token || null;
    })
    .catch(() => {
      cached = null;
    });
  return {
    get: () => cached,
    set: (_v: string) => {
      // No-op: callers go through saveSettings(...) which persists
      // to AsyncStorage and updates the cache on the next read.
    },
    clear: () => {
      cached = null;
    },
  };
}

const client: ApiClient = createClient({
  baseUrl: DEFAULT_BASE_URL,
  tokenStore: asyncTokenStore(),
});

// resolveBaseUrl reads the saved base URL from settings, falling back
// to the default. It updates the module-level client's base URL so
// all calls use the same instance.
async function resolveBaseUrl(): Promise<string> {
  try {
    const s = await getSettings();
    client.setBaseUrl(s.serverUrl || DEFAULT_BASE_URL);
    return s.serverUrl || DEFAULT_BASE_URL;
  } catch {
    return DEFAULT_BASE_URL;
  }
}

// refreshToken re-reads the token from AsyncStorage. Call this at the
// start of every write call to make sure the client's cached token
// matches the latest saved value.
async function refreshToken(): Promise<void> {
  const s = await getSettings();
  client.setToken(s.token || '');
}

export async function checkHealth(url?: string): Promise<boolean> {
  if (url) client.setBaseUrl(url);
  else await resolveBaseUrl();
  try {
    await client.checkHealth();
    return true;
  } catch {
    return false;
  }
}

export async function sendCapture(
  payload: Omit<Capture, 'id' | 'synced' | 'createdAt'>,
): Promise<void> {
  await refreshToken();
  await resolveBaseUrl();
  await client.capture({
    type: payload.type,
    title: payload.title,
    text: payload.text,
    project: payload.project,
    tags: payload.tags,
  });
}

export async function searchVault(query: string, url?: string): Promise<SearchResult[]> {
  if (url) client.setBaseUrl(url);
  else await resolveBaseUrl();
  if (!query.trim()) return [];
  return client.search({ q: query });
}

export async function getProjects(url?: string): Promise<string[]> {
  if (url) client.setBaseUrl(url);
  else await resolveBaseUrl();
  return client.getProjects();
}

export async function verifyToken(url?: string): Promise<AuthVerifyResponse | null> {
  if (url) client.setBaseUrl(url);
  else await resolveBaseUrl();
  await refreshToken();
  try {
    return await client.verifyAuth();
  } catch {
    return null;
  }
}
