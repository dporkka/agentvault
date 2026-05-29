import type {
  HealthResponse,
  VaultStatus,
  SearchResult,
  Note,
  AskResponse,
  CreateNoteData,
  CreateNoteResponse,
} from './types';

const DEFAULT_URL = 'http://127.0.0.1:47321';
const STORAGE_KEY_URL = 'agentvault_server_url';
const STORAGE_KEY_TOKEN = 'agentvault_token';

export class ApiError extends Error {
  status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
  }
}

class ApiClient {
  private _baseUrl: string;
  private _token: string;

  constructor(baseUrl?: string, token?: string) {
    this._baseUrl = baseUrl || localStorage.getItem(STORAGE_KEY_URL) || DEFAULT_URL;
    this._token = token || localStorage.getItem(STORAGE_KEY_TOKEN) || '';
  }

  get baseUrl(): string {
    return this._baseUrl;
  }

  set baseUrl(url: string) {
    this._baseUrl = url.replace(/\/$/, '');
    localStorage.setItem(STORAGE_KEY_URL, this._baseUrl);
  }

  get token(): string {
    return this._token;
  }

  set token(t: string) {
    this._token = t;
    if (t) {
      localStorage.setItem(STORAGE_KEY_TOKEN, t);
    } else {
      localStorage.removeItem(STORAGE_KEY_TOKEN);
    }
  }

  private async fetch<T>(path: string, options?: RequestInit & { skipAuth?: boolean }): Promise<T> {
    const url = `${this._baseUrl}${path}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...((options?.headers as Record<string, string>) || {}),
    };

    if (this._token && !options?.skipAuth) {
      headers['X-AgentVault-Token'] = this._token;
    }

    let response: Response;
    try {
      response = await fetch(url, {
        ...options,
        headers,
      });
    } catch (err) {
      throw new ApiError(
        `Cannot connect to AgentVault server at ${this._baseUrl}. Is the server running?`,
        0
      );
    }

    if (!response.ok) {
      let message = `HTTP ${response.status}`;
      try {
        const body = await response.json();
        message = body.error || body.message || message;
      } catch {
        try {
          const text = await response.text();
          if (text) message = text;
        } catch { /* ignore */ }
      }
      throw new ApiError(message, response.status);
    }

    // Handle 204 No Content
    if (response.status === 204) {
      return undefined as T;
    }

    try {
      return (await response.json()) as T;
    } catch {
      return undefined as T;
    }
  }

  // === Public API Methods ===

  async checkHealth(): Promise<HealthResponse> {
    return this.fetch<HealthResponse>('/health', { skipAuth: true });
  }

  async getVaultStatus(): Promise<VaultStatus> {
    return this.fetch<VaultStatus>('/vault/status');
  }

  async search(query: string, type?: string, project?: string, limit = 20): Promise<SearchResult[]> {
    const params = new URLSearchParams({ q: query, limit: String(limit) });
    if (type) params.set('type', type);
    if (project) params.set('project', project);
    return this.fetch<SearchResult[]>(`/search?${params.toString()}`);
  }

  async getNote(id: string): Promise<Note> {
    return this.fetch<Note>(`/notes/${encodeURIComponent(id)}`);
  }

  async createNote(data: CreateNoteData): Promise<CreateNoteResponse> {
    return this.fetch<CreateNoteResponse>('/notes', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async ask(question: string): Promise<AskResponse> {
    return this.fetch<AskResponse>('/ask', {
      method: 'POST',
      body: JSON.stringify({ question }),
    });
  }

  async getProjects(): Promise<string[]> {
    return this.fetch<string[]>('/projects');
  }

  async getRecent(limit = 10): Promise<SearchResult[]> {
    return this.fetch<SearchResult[]>(`/recent?limit=${limit}`);
  }

  async triggerIndex(): Promise<void> {
    await this.fetch<void>('/vault/index', { method: 'POST' });
  }
}

export const api = new ApiClient();
