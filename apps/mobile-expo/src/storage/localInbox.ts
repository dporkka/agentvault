import AsyncStorage from '@react-native-async-storage/async-storage';
import type { Capture, AppSettings, CaptureSyncStatus } from '../types';
import { DEFAULT_APP_SETTINGS, loadSettings, persistSettings } from './settingsStore';

const INBOX_KEY = 'agentvault_inbox';
const INBOX_SCHEMA_VERSION = 2;
const SCHEMA_VERSION_KEY = 'agentvault_inbox_schema_version';

function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

function normalizeCapture(capture: Capture): Capture {
  return {
    ...capture,
    syncStatus: capture.synced ? 'synced' : (capture.syncStatus ?? 'unsynced'),
  };
}

async function getSchemaVersion(): Promise<number> {
  try {
    const v = await AsyncStorage.getItem(SCHEMA_VERSION_KEY);
    return v ? parseInt(v, 10) : 1;
  } catch {
    return 1;
  }
}

async function setSchemaVersion(version: number): Promise<void> {
  await AsyncStorage.setItem(SCHEMA_VERSION_KEY, String(version));
}

async function migrateCaptures(captures: Capture[]): Promise<Capture[]> {
  // v1 -> v2: ensure syncStatus, retryCount, and lastRetryAt fields exist.
  return captures.map((c) => ({
    ...c,
    syncStatus: c.synced ? 'synced' : (c.syncStatus ?? 'unsynced'),
    retryCount: c.retryCount ?? 0,
  }));
}

export async function addCapture(
  capture: Omit<Capture, 'id' | 'createdAt' | 'synced' | 'syncStatus' | 'retryCount'>,
): Promise<Capture> {
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
    if (!Array.isArray(parsed)) {
      console.warn('[localInbox] Inbox data is not an array; resetting to empty.');
      return [];
    }
    const currentVersion = await getSchemaVersion();
    let captures = parsed.map(normalizeCapture);
    if (currentVersion < INBOX_SCHEMA_VERSION) {
      captures = await migrateCaptures(captures);
      await AsyncStorage.setItem(INBOX_KEY, JSON.stringify(captures));
      await setSchemaVersion(INBOX_SCHEMA_VERSION);
    }
    return captures;
  } catch (err) {
    console.error('[localInbox] Failed to parse inbox data:', err);
    return [];
  }
}

export async function getUnsyncedCaptures(): Promise<Capture[]> {
  const captures = await getCaptures();
  return captures.filter((c) => !c.synced && c.syncStatus !== 'syncing');
}

export async function updateCapture(id: string, patch: Partial<Capture>): Promise<void> {
  const captures = await getCaptures();
  const updated = captures.map((c) => (c.id === id ? { ...c, ...patch } : c));
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
    lastRetryAt: new Date().toISOString(),
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
  await persistSettings(settings);
}

export async function getSettings(): Promise<AppSettings> {
  try {
    return await loadSettings();
  } catch {
    return DEFAULT_APP_SETTINGS;
  }
}

export async function clearInbox(): Promise<void> {
  await AsyncStorage.removeItem(INBOX_KEY);
}
