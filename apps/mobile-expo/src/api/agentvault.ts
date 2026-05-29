import type { Capture, SearchResult } from '../types';
import { getSettings } from '../storage/localInbox';

const DEFAULT_URL = 'http://127.0.0.1:47321';

async function getBaseUrl(): Promise<string> {
  const settings = await getSettings();
  return settings.serverUrl || DEFAULT_URL;
}

async function fetchWithTimeout(
  url: string,
  options: RequestInit = {},
  timeoutMs = 5000
): Promise<Response> {
  const controller = new AbortController();
  const timeout = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetch(url, { ...options, signal: controller.signal });
    clearTimeout(timeout);
    return res;
  } catch (err) {
    clearTimeout(timeout);
    throw err;
  }
}

export async function checkHealth(url?: string): Promise<boolean> {
  try {
    const baseUrl = url || (await getBaseUrl());
    const res = await fetchWithTimeout(`${baseUrl}/health`, {}, 3000);
    return res.ok;
  } catch {
    return false;
  }
}

export async function sendCapture(
  payload: Omit<Capture, 'id' | 'synced' | 'createdAt'>
): Promise<void> {
  const settings = await getSettings();
  const baseUrl = settings.serverUrl || DEFAULT_URL;
  // /capture is a write endpoint; the server requires the auth token.
  const headers: Record<string, string> = { 'Content-Type': 'application/json' };
  if (settings.token) {
    headers['X-AgentVault-Token'] = settings.token;
  }
  const res = await fetchWithTimeout(`${baseUrl}/capture`, {
    method: 'POST',
    headers,
    body: JSON.stringify(payload),
  });
  if (!res.ok) {
    throw new Error(`Server returned ${res.status}`);
  }
}

export async function searchVault(
  query: string,
  url?: string
): Promise<SearchResult[]> {
  const baseUrl = url || (await getBaseUrl());
  const res = await fetchWithTimeout(
    `${baseUrl}/search?q=${encodeURIComponent(query)}`,
    {},
    5000
  );
  if (!res.ok) {
    throw new Error(`Server returned ${res.status}`);
  }
  const data = (await res.json()) as SearchResult[];
  return data;
}

export async function getProjects(url?: string): Promise<string[]> {
  const baseUrl = url || (await getBaseUrl());
  const res = await fetchWithTimeout(`${baseUrl}/projects`, {}, 5000);
  if (!res.ok) {
    throw new Error(`Server returned ${res.status}`);
  }
  const data = (await res.json()) as string[];
  return data;
}
