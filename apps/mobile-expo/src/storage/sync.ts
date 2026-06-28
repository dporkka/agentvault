import { sendCapture } from '../api/agentvault';
import type { Capture } from '../types';
import { getUnsyncedCaptures, markAsSynced, markAsSyncing, markAsFailed } from './localInbox';

export interface SyncResult {
  sent: number;
  failed: number;
  skipped: number;
  errors: { id: string; title: string; error: string }[];
}

export interface SyncOptions {
  /** Continue syncing remaining captures after a failure. Defaults to true. */
  continueOnError?: boolean;
  /** Optional filter to sync a specific capture. */
  captureId?: string;
  /** Ignore exponential backoff and retry failed captures immediately. */
  force?: boolean;
}

const BASE_BACKOFF_MS = 5000;
const MAX_BACKOFF_MS = 5 * 60 * 1000; // 5 minutes

function getBackoffDelayMs(retryCount: number): number {
  return Math.min(MAX_BACKOFF_MS, BASE_BACKOFF_MS * Math.pow(2, retryCount - 1));
}

function isBackoffElapsed(capture: Capture): boolean {
  if (!capture.lastRetryAt || !capture.retryCount) return true;
  const elapsed = Date.now() - new Date(capture.lastRetryAt).getTime();
  return elapsed >= getBackoffDelayMs(capture.retryCount);
}

/**
 * Sync unsynced captures to the AgentVault server.
 *
 * Updates each capture's syncStatus through the lifecycle
 * (unsynced -> syncing -> synced | failed) and returns a summary.
 *
 * TODO: Pass the local capture `id` as an idempotency key once the server
 * `/capture` endpoint accepts an optional `externalId` field. Until then,
 * retries may create duplicate inbox files on the server.
 */
export async function syncCaptures(options: SyncOptions = {}): Promise<SyncResult> {
  const { continueOnError = true, captureId, force = false } = options;
  const captures = captureId
    ? (await getUnsyncedCaptures()).filter((c) => c.id === captureId)
    : await getUnsyncedCaptures();

  const result: SyncResult = { sent: 0, failed: 0, skipped: 0, errors: [] };

  for (const cap of captures) {
    if (!force && !isBackoffElapsed(cap)) {
      result.skipped++;
      continue;
    }

    await markAsSyncing(cap.id);
    try {
      await sendCapture({
        type: cap.type,
        title: cap.title,
        text: cap.text,
        project: cap.project,
        tags: cap.tags,
      });
      await markAsSynced(cap.id);
      result.sent++;
    } catch (err) {
      const message = err instanceof Error ? err.message : 'Sync failed';
      await markAsFailed(cap.id, message);
      result.failed++;
      result.errors.push({ id: cap.id, title: cap.title, error: message });
      if (!continueOnError) break;
    }
  }

  return result;
}

/**
 * Build a user-facing summary message from a sync result.
 */
export function formatSyncResult(result: SyncResult): string {
  if (result.sent > 0 && result.failed === 0) {
    return `Synced ${result.sent} capture${result.sent === 1 ? '' : 's'}`;
  }
  if (result.sent > 0 && result.failed > 0) {
    return `Synced ${result.sent}; ${result.failed} failed`;
  }
  if (result.failed > 0) {
    return `Sync failed for ${result.failed} capture${result.failed === 1 ? '' : 's'}`;
  }
  if (result.skipped > 0) {
    return `${result.skipped} capture${result.skipped === 1 ? '' : 's'} skipped (backing off)`;
  }
  return 'Nothing to sync';
}

/**
 * Determine whether a capture is eligible to be synced.
 */
export function isSyncable(capture: Capture): boolean {
  return !capture.synced && capture.syncStatus !== 'syncing';
}

/**
 * Determine whether a capture can be retried now, respecting backoff.
 */
export function canRetry(capture: Capture): boolean {
  return isSyncable(capture) && isBackoffElapsed(capture);
}
