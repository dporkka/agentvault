export interface Capture {
  id: string;
  type: 'text' | 'voice' | 'photo';
  title: string;
  text?: string;
  project?: string;
  tags: string[];
  createdAt: string;
  synced: boolean;
}

export interface SearchResult {
  id: string;
  title: string;
  path: string;
  type: string;
  snippet: string;
}

export interface AppSettings {
  serverUrl: string;
  defaultProject: string;
  token: string;
}
