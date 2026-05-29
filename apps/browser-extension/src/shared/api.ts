import type { CapturePayload, SearchResult } from './types';

const API_BASE = 'http://127.0.0.1:47321';
const TOKEN_KEY = 'agentvault_token';

// getToken reads the saved auth token from extension storage. Write endpoints
// (e.g. /capture) require it in the X-AgentVault-Token header.
export async function getToken(): Promise<string> {
  try {
    const result = await chrome.storage.local.get(TOKEN_KEY);
    return (result[TOKEN_KEY] as string) || '';
  } catch {
    return '';
  }
}

export async function setToken(token: string): Promise<void> {
  await chrome.storage.local.set({ [TOKEN_KEY]: token });
}

export async function checkHealth(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/health`, { method: 'GET', mode: 'cors' });
    return res.ok;
  } catch {
    return false;
  }
}

export async function sendCapture(payload: CapturePayload): Promise<{ path: string }> {
  try {
    const token = await getToken();
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (token) {
      headers['X-AgentVault-Token'] = token;
    }
    const res = await fetch(`${API_BASE}/capture`, {
      method: 'POST',
      headers,
      mode: 'cors',
      body: JSON.stringify(payload),
    });
    if (!res.ok) {
      const errText = await res.text().catch(() => 'Unknown error');
      throw new Error(`Server error ${res.status}: ${errText}`);
    }
    return await res.json() as { path: string };
  } catch (err) {
    if (err instanceof TypeError && err.message.includes('fetch')) {
      throw new Error('AgentVault server is not running. Start it with: agentvault serve');
    }
    throw err;
  }
}

export async function searchVault(query: string): Promise<SearchResult[]> {
  if (!query.trim()) return [];
  try {
    const res = await fetch(`${API_BASE}/search?q=${encodeURIComponent(query)}`, { method: 'GET', mode: 'cors' });
    if (!res.ok) {
      const errText = await res.text().catch(() => 'Unknown error');
      throw new Error(`Server error ${res.status}: ${errText}`);
    }
    return await res.json() as SearchResult[];
  } catch (err) {
    if (err instanceof TypeError && err.message.includes('fetch')) {
      throw new Error('AgentVault server is not running. Start it with: agentvault serve');
    }
    throw err;
  }
}

export async function getProjects(): Promise<string[]> {
  try {
    const res = await fetch(`${API_BASE}/projects`, { method: 'GET', mode: 'cors' });
    if (!res.ok) return [];
    return await res.json() as string[];
  } catch {
    return [];
  }
}
