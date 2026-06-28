// Server-facing types come from @agentvault/contract. Local shapes that
// the mobile app persists in AsyncStorage (Capture, AppSettings) stay
// here because they have nothing to do with the HTTP contract.

export type { SearchResult } from '@agentvault/contract';

export type CaptureSyncStatus = 'unsynced' | 'syncing' | 'synced' | 'failed';

export interface Capture {
  id: string;
  type: 'text' | 'voice' | 'photo';
  title: string;
  text?: string;
  project?: string;
  tags: string[];
  createdAt: string;
  synced: boolean;
  syncStatus?: CaptureSyncStatus;
  syncError?: string;
  retryCount?: number;
  lastRetryAt?: string;
}

export interface AppSettings {
  serverUrl: string;
  defaultProject: string;
  token: string;
}
