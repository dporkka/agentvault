import AsyncStorage from '@react-native-async-storage/async-storage';
import type { Capture, AppSettings } from '../types';

const INBOX_KEY = 'agentvault_inbox';
const SETTINGS_KEY = 'agentvault_settings';

function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
}

export async function addCapture(capture: Omit<Capture, 'id' | 'createdAt' | 'synced'>): Promise<Capture> {
  const newCapture: Capture = {
    ...capture,
    id: generateId(),
    createdAt: new Date().toISOString(),
    synced: false,
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
    return Array.isArray(parsed) ? parsed : [];
  } catch {
    return [];
  }
}

export async function getUnsyncedCaptures(): Promise<Capture[]> {
  const captures = await getCaptures();
  return captures.filter((c) => !c.synced);
}

export async function markAsSynced(id: string): Promise<void> {
  const captures = await getCaptures();
  const updated = captures.map((c) =>
    c.id === id ? { ...c, synced: true } : c
  );
  await AsyncStorage.setItem(INBOX_KEY, JSON.stringify(updated));
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
    return { serverUrl: 'http://127.0.0.1:47321', defaultProject: '', token: '' };
  }
  try {
    return JSON.parse(data) as AppSettings;
  } catch {
    return { serverUrl: 'http://127.0.0.1:47321', defaultProject: '', token: '' };
  }
}

export async function clearInbox(): Promise<void> {
  await AsyncStorage.removeItem(INBOX_KEY);
}
