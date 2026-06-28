// Lightweight offline capture queue for the browser extension.
// When a capture fails because the server is unreachable or the user is not
// authenticated, the payload is stored in chrome.storage.local. The background
// service worker retries queued captures periodically, and the popup can also
// trigger a retry manually.

import { sendCapture, syncClientConfig } from './api';
import type { CapturePayload } from './local';
import { classifyError } from './errors';

const QUEUE_KEY = 'agentvault_capture_queue';

export type CaptureSyncState = 'unsynced' | 'syncing' | 'synced' | 'failed';

export interface QueuedCapture {
  id: string;
  payload: CapturePayload;
  state: CaptureSyncState;
  attempts: number;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
}

export interface CaptureResult {
  state: CaptureSyncState;
  path?: string;
  error?: string;
  queued?: boolean;
}

function makeId(): string {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

async function getQueue(): Promise<QueuedCapture[]> {
  const result = await new Promise<Record<string, unknown>>((resolve) => {
    try {
      chrome.storage.local.get(QUEUE_KEY, (r) => resolve(r || {}));
    } catch {
      resolve({});
    }
  });
  const queue = result?.[QUEUE_KEY];
  return Array.isArray(queue) ? queue : [];
}

async function setQueue(queue: QueuedCapture[]): Promise<void> {
  await new Promise<void>((resolve) => {
    try {
      chrome.storage.local.set({ [QUEUE_KEY]: queue }, () => resolve());
    } catch {
      resolve();
    }
  });
}

export async function getPendingCount(): Promise<number> {
  const queue = await getQueue();
  return queue.filter((item) => item.state === 'unsynced' || item.state === 'failed').length;
}

export async function listQueuedCaptures(): Promise<QueuedCapture[]> {
  return getQueue();
}

/**
 * Try to send a capture immediately. If the send fails with a recoverable
 * error (network or auth), queue it for retry and return state 'unsynced'.
 */
export async function sendOrQueueCapture(payload: CapturePayload): Promise<CaptureResult> {
  try {
    await syncClientConfig();
    const response = await sendCapture(payload);
    return { state: 'synced', path: response.path };
  } catch (err) {
    const classified = classifyError(err);
    if (!classified.recoverable) {
      return { state: 'failed', error: classified.message, queued: false };
    }
    const queue = await getQueue();
    queue.push({
      id: makeId(),
      payload,
      state: 'unsynced',
      attempts: 1,
      lastError: classified.message,
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    });
    await setQueue(queue);
    return { state: 'unsynced', error: classified.message, queued: true };
  }
}

/**
 * Retry all queued captures. Returns the number successfully synced.
 */
export async function retryQueuedCaptures(): Promise<number> {
  const queue = await getQueue();
  if (queue.length === 0) return 0;

  let synced = 0;
  const now = new Date().toISOString();
  const remaining: QueuedCapture[] = [];

  for (const item of queue) {
    if (item.state === 'synced') {
      synced++;
      continue;
    }
    try {
      await syncClientConfig();
      const response = await sendCapture(item.payload);
      synced++;
      item.state = 'synced';
      item.lastError = undefined;
      // Keep recently synced items briefly so callers can see the count drop.
      item.updatedAt = now;
      remaining.push(item);
    } catch (err) {
      const classified = classifyError(err);
      item.state = classified.recoverable && item.attempts < 5 ? 'unsynced' : 'failed';
      item.attempts += 1;
      item.lastError = classified.message;
      item.updatedAt = now;
      remaining.push(item);
    }
  }

  await setQueue(remaining);
  return synced;
}

/**
 * Remove a queued capture by id, or clear all synced captures when no id is given.
 */
export async function removeQueuedCapture(id?: string): Promise<void> {
  if (!id) {
    const queue = await getQueue();
    await setQueue(queue.filter((item) => item.state !== 'synced'));
    return;
  }
  const queue = await getQueue();
  await setQueue(queue.filter((item) => item.id !== id));
}
