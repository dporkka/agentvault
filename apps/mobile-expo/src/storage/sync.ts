import { sendCapture } from '../api/agentvault';
import type { Capture } from '../types';
import {
  getUnsyncedCaptures,
  markAsSynced,
  markAsSyncing,
  markAsFailed,
} from './localInbox';

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
}

/**
 * Sync unsynced captures to the AgentVault server.
 *
 * Updates each capture's syncStatus through the lifecycle
 * (unsynced -> syncing -> synced | failed) and returns a summary.
 */
export async function syncCaptures(options: SyncOptions = {}): Promise<SyncResult> {
  const { continueOnError = true, captureId } = options;
  const captures = captureId
    ? (await getUnsyncedCaptures()).filter((c) => c.id === captureId)
    : await getUnsyncedCaptures();

  const result: SyncResult = { sent: 0, failed: 0, skipped: 0, errors: [] };

  for (const cap of captures) {
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
  return 'Nothing to sync';
}

/**
 * Determine whether a capture is eligible to be synced.
 */
export function isSyncable(capture: Capture): boolean {
  return !capture.synced && capture.syncStatus !== 'syncing';
}
