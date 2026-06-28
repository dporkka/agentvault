// Thin wrapper over @agentvault/contract's HTTP client. The mobile app
// reads the base URL and auth token from AsyncStorage (via the existing
// localInbox settings store); the contract package stays portable by
// accepting a TokenStore from the caller.

import {
  createClient,
  type ApiClient,
  type AuthVerifyResponse,
  type NoteDetail,
  type SearchParams,
  type SearchResult,
  DEFAULT_BASE_URL,
} from '@agentvault/contract';
import { getSettings } from '../storage/localInbox';
import type { Capture } from '../types';

export { DEFAULT_BASE_URL };

// Mutable client configuration. SettingsContext is the source of truth and
// calls updateClientConfig whenever settings load or change. API helper
// functions also refresh from AsyncStorage before authenticated calls as a
// safety net when components render before the context has loaded.
let currentToken = '';
let currentBaseUrl = DEFAULT_BASE_URL;

function tokenStore() {
  return {
    get: () => currentToken || null,
    set: (v: string) => {
      currentToken = v;
    },
    clear: () => {
      currentToken = '';
    },
  };
}

const client: ApiClient = createClient({
  baseUrl: currentBaseUrl,
  tokenStore: tokenStore(),
});

export function updateClientConfig(baseUrl?: string, token?: string): void {
  if (baseUrl !== undefined) {
    currentBaseUrl = baseUrl.replace(/\/$/, '');
    client.setBaseUrl(currentBaseUrl);
  }
  if (token !== undefined) {
    currentToken = token;
    client.setToken(token);
  }
}

// resolveBaseUrl ensures the module-level client uses the latest saved server
// URL. Prefer using SettingsContext directly; this is a fallback for code
// that has not yet been migrated.
async function resolveBaseUrl(): Promise<string> {
  try {
    const s = await getSettings();
    updateClientConfig(s.serverUrl, undefined);
    return currentBaseUrl;
  } catch {
    return currentBaseUrl;
  }
}

// refreshToken re-reads the token from AsyncStorage. Call this at the start
// of every write call to make sure the client's cached token matches the
// latest saved value.
async function refreshToken(): Promise<void> {
  const s = await getSettings();
  updateClientConfig(undefined, s.token);
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

export async function searchVault(
  query: string | (SearchParams & { q?: string }),
  url?: string,
): Promise<SearchResult[]> {
  if (url) client.setBaseUrl(url);
  else await resolveBaseUrl();

  const params: SearchParams = typeof query === 'string' ? { q: query } : query;
  const q = params.q ?? '';
  if (!String(q).trim()) return [];
  return client.search(params);
}

export async function getProjects(url?: string): Promise<string[]> {
  if (url) client.setBaseUrl(url);
  else await resolveBaseUrl();
  return client.getProjects();
}

export async function getNote(id: string, url?: string): Promise<NoteDetail> {
  if (url) client.setBaseUrl(url);
  else await resolveBaseUrl();
  return client.getNote(id);
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
