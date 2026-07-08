// Thin wrapper over @agentvault/contract's HTTP client. The mobile app
// creates a fresh client for every request from the persisted settings, so
// there is no module-level mutable state and concurrent calls cannot race.

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

interface ClientOverrides {
  baseUrl?: string;
  token?: string;
}

/**
 * Build a new API client from persisted settings. Callers may override the
 * base URL or token for one-off checks (e.g., testing a server URL before
 * saving it).
 */
export async function createMobileClient(overrides?: ClientOverrides): Promise<ApiClient> {
  const settings = await getSettings();
  const baseUrl = (overrides?.baseUrl ?? (settings.serverUrl || DEFAULT_BASE_URL)).replace(/\/$/, '');
  const token = overrides?.token ?? settings.token ?? '';
  return createClient({ baseUrl, token });
}

export async function checkHealth(url?: string): Promise<boolean> {
  try {
    const client = await createMobileClient(url ? { baseUrl: url } : undefined);
    await client.checkHealth();
    return true;
  } catch {
    return false;
  }
}

export async function sendCapture(
  payload: Omit<Capture, 'id' | 'synced' | 'createdAt'>,
): Promise<void> {
  const client = await createMobileClient();
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
  const client = await createMobileClient(url ? { baseUrl: url } : undefined);

  const params: SearchParams = typeof query === 'string' ? { q: query } : query;
  const q = params.q ?? '';
  if (!String(q).trim()) return [];
  return client.search(params);
}

export async function getProjects(url?: string): Promise<string[]> {
  const client = await createMobileClient(url ? { baseUrl: url } : undefined);
  return client.getProjects();
}

export async function getNote(id: string, url?: string): Promise<NoteDetail> {
  const client = await createMobileClient(url ? { baseUrl: url } : undefined);
  return client.getNote(id);
}

export async function getRecentNotes(limit?: number, url?: string): Promise<SearchResult[]> {
  const client = await createMobileClient(url ? { baseUrl: url } : undefined);
  return client.getRecent({ limit });
}

export async function verifyToken(url?: string): Promise<AuthVerifyResponse | null> {
  try {
    const client = await createMobileClient(url ? { baseUrl: url } : undefined);
    return await client.verifyAuth();
  } catch {
    return null;
  }
}
