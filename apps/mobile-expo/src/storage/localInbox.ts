import AsyncStorage from '@react-native-async-storage/async-storage';
import { DEFAULT_BASE_URL } from '@agentvault/contract';
import type { Capture, AppSettings, CaptureSyncStatus } from '../types';

const INBOX_KEY = 'agentvault_inbox';
const SETTINGS_KEY = 'agentvault_settings';

function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

function normalizeCapture(capture: Capture): Capture {
  return {
    ...capture,
    syncStatus: capture.synced ? 'synced' : (capture.syncStatus ?? 'unsynced'),
  };
}

export async function addCapture(capture: Omit<Capture, 'id' | 'createdAt' | 'synced' | 'syncStatus' | 'retryCount'>): Promise<Capture> {
  const newCapture: Capture = {
    ...capture,
    id: generateId(),
    createdAt: new Date().toISOString(),
    synced: false,
    syncStatus: 'unsynced',
    retryCount: 0,
  };

  const existing = await getCaptures();
  const updated = [newCapture, ...existing];
  await AsyncStorage.setItem(INBOX_KEY, JSON.stringify(updated));
  return newCapture;
}

export async function getCaptures(): Promise<Capture[]> {
  const data = await AsyncStorage.getItem(INBOX_KEY);
  if (!data) return [];
  try {
    const parsed = JSON.parse(data) as Capture[];
    return Array.isArray(parsed) ? parsed.map(normalizeCapture) : [];
  } catch {
    return [];
  }
}

export async function getUnsyncedCaptures(): Promise<Capture[]> {
  const captures = await getCaptures();
  return captures.filter((c) => !c.synced && c.syncStatus !== 'syncing');
}

export async function updateCapture(id: string, patch: Partial<Capture>): Promise<void> {
  const captures = await getCaptures();
  const updated = captures.map((c) =>
    c.id === id ? { ...c, ...patch } : c
  );
  await AsyncStorage.setItem(INBOX_KEY, JSON.stringify(updated));
}

export async function markAsSynced(id: string): Promise<void> {
  await updateCapture(id, { synced: true, syncStatus: 'synced', syncError: undefined });
}

export async function markAsSyncing(id: string): Promise<void> {
  await updateCapture(id, { syncStatus: 'syncing', syncError: undefined });
}

export async function markAsFailed(id: string, error: string): Promise<void> {
  const captures = await getCaptures();
  const capture = captures.find((c) => c.id === id);
  const retryCount = (capture?.retryCount ?? 0) + 1;
  await updateCapture(id, {
    synced: false,
    syncStatus: 'failed',
    syncError: error,
    retryCount,
  });
}

export async function setSyncStatus(id: string, status: CaptureSyncStatus): Promise<void> {
  await updateCapture(id, { syncStatus: status });
}

export async function deleteCapture(id: string): Promise<void> {
  const captures = await getCaptures();
  const updated = captures.filter((c) => c.id !== id);
  await AsyncStorage.setItem(INBOX_KEY, JSON.stringify(updated));
}

export async function saveSettings(settings: AppSettings): Promise<void> {
  await AsyncStorage.setItem(SETTINGS_KEY, JSON.stringify(settings));
}

export async function getSettings(): Promise<AppSettings> {
  const data = await AsyncStorage.getItem(SETTINGS_KEY);
  if (!data) {
    return { serverUrl: DEFAULT_BASE_URL, defaultProject: '', token: '' };
  }
  try {
    return JSON.parse(data) as AppSettings;
  } catch {
    return { serverUrl: DEFAULT_BASE_URL, defaultProject: '', token: '' };
  }
}

export async function clearInbox(): Promise<void> {
  await AsyncStorage.removeItem(INBOX_KEY);
}
