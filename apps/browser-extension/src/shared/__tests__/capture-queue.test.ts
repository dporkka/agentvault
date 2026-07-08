import { describe, it, expect, vi, beforeEach } from 'vitest';
import { installChromeStorageMock } from './storage-mock';

const mockSendCapture = vi.hoisted(() => vi.fn());
const mockSyncClientConfig = vi.hoisted(() => vi.fn());

vi.mock('../api', () => ({
  sendCapture: mockSendCapture,
  syncClientConfig: mockSyncClientConfig,
}));

import {
  sendOrQueueCapture,
  retryQueuedCaptures,
  removeQueuedCapture,
  getPendingCount,
  listQueuedCaptures,
} from '../capture-queue';

describe('capture-queue', () => {
  beforeEach(() => {
    installChromeStorageMock();
    mockSendCapture.mockReset();
    mockSyncClientConfig.mockReset();
    mockSyncClientConfig.mockResolvedValue(undefined);
  });

  const payload = {
    type: 'webpage' as const,
    title: 'Test',
    url: 'http://example.com',
    capturedAt: new Date().toISOString(),
  };

  describe('sendOrQueueCapture', () => {
    it('returns synced when the server accepts the capture', async () => {
      mockSendCapture.mockResolvedValue({ path: 'notes/test.md' });
      const result = await sendOrQueueCapture(payload);
      expect(result.state).toBe('synced');
      expect(result.path).toBe('notes/test.md');
      expect(await getPendingCount()).toBe(0);
    });

    it('queues a recoverable failure and reports it as unsynced', async () => {
      mockSendCapture.mockRejectedValue(new Error('Network unreachable'));
      const result = await sendOrQueueCapture(payload);
      expect(result.state).toBe('unsynced');
      expect(result.queued).toBe(true);
      expect(result.error).toContain('Network');
      expect(await getPendingCount()).toBe(1);
      const queue = await listQueuedCaptures();
      expect(queue).toHaveLength(1);
      expect(queue[0].payload.title).toBe('Test');
      expect(queue[0].attempts).toBe(1);
    });

    it('does not queue an unrecoverable failure', async () => {
      const err = new Error('Bad request');
      // Make classifyError treat this as client/non-recoverable by shape:
      // generic Error with non-network message is unknown/non-recoverable.
      mockSendCapture.mockRejectedValue(err);
      const result = await sendOrQueueCapture(payload);
      expect(result.state).toBe('failed');
      expect(result.queued).toBe(false);
      expect(await getPendingCount()).toBe(0);
    });
  });

  describe('retryQueuedCaptures', () => {
    it('syncs queued items and leaves them briefly in the queue', async () => {
      mockSendCapture.mockRejectedValueOnce(new Error('Network error')).mockResolvedValue({ path: 'ok.md' });
      await sendOrQueueCapture(payload);

      mockSendCapture.mockReset();
      mockSendCapture.mockResolvedValue({ path: 'ok.md' });
      const synced = await retryQueuedCaptures();
      expect(synced).toBe(1);
      const queue = await listQueuedCaptures();
      expect(queue[0].state).toBe('synced');
      expect(await getPendingCount()).toBe(0);
    });

    it('increments attempts for failed retries and marks permanent after 5 tries', async () => {
      mockSendCapture.mockRejectedValue(new Error('Network error'));
      await sendOrQueueCapture(payload);

      for (let i = 0; i < 5; i++) {
        await retryQueuedCaptures();
      }
      const queue = await listQueuedCaptures();
      expect(queue[0].state).toBe('failed');
      expect(queue[0].attempts).toBeGreaterThanOrEqual(5);
    });
  });

  describe('removeQueuedCapture', () => {
    it('removes a single queued item by id', async () => {
      mockSendCapture.mockRejectedValue(new Error('Network error'));
      await sendOrQueueCapture(payload);
      const queue = await listQueuedCaptures();
      await removeQueuedCapture(queue[0].id);
      expect(await listQueuedCaptures()).toHaveLength(0);
    });

    it('clears synced items when called without an id', async () => {
      mockSendCapture.mockRejectedValueOnce(new Error('Network error')).mockResolvedValue({ path: 'ok.md' });
      await sendOrQueueCapture(payload);
      mockSendCapture.mockReset();
      mockSendCapture.mockResolvedValue({ path: 'ok.md' });
      await retryQueuedCaptures();
      expect((await listQueuedCaptures()).length).toBeGreaterThan(0);
      await removeQueuedCapture();
      expect(await listQueuedCaptures()).toHaveLength(0);
    });
  });
});
