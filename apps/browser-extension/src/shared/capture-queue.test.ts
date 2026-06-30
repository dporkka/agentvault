import { describe, it, expect, vi, beforeEach } from 'vitest';
import { ApiError } from '@agentvault/contract';
import { sendCapture, syncClientConfig } from './api';
import {
  sendOrQueueCapture,
  retryQueuedCaptures,
  getPendingCount,
  listQueuedCaptures,
  removeQueuedCapture,
} from './capture-queue';

vi.mock('./api', () => ({
  sendCapture: vi.fn(),
  syncClientConfig: vi.fn(),
}));

const QUEUE_KEY = 'agentvault_capture_queue';

interface StorageMock {
  get: ReturnType<typeof vi.fn>;
  set: ReturnType<typeof vi.fn>;
}

function createChromeStorageMock() {
  const store: Record<string, unknown> = {};
  const storage: StorageMock = {
    get: vi.fn((keys: string | string[] | null, callback?: (items: Record<string, unknown>) => void) => {
      const keyList =
        typeof keys === 'string'
          ? [keys]
          : Array.isArray(keys)
            ? keys
            : Object.keys(keys ?? {});
      const result: Record<string, unknown> = {};
      for (const key of keyList) {
        if (key in store) {
          result[key] = store[key];
        }
      }
      if (callback) callback(result);
    }),
    set: vi.fn((items: Record<string, unknown>, callback?: () => void) => {
      Object.assign(store, items);
      if (callback) callback();
    }),
  };
  return { store, storage };
}

const { store, storage } = createChromeStorageMock();

vi.stubGlobal('chrome', {
  storage: {
    local: storage,
  },
});

function makePayload(url = 'https://example.com/article'): import('./local').CapturePayload {
  return {
    type: 'webpage',
    title: 'Example',
    url,
    text: 'Some page text',
    capturedAt: new Date().toISOString(),
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  Object.keys(store).forEach((key) => delete store[key]);
  vi.mocked(syncClientConfig).mockResolvedValue(undefined);
  vi.mocked(sendCapture).mockReset();
});

describe('sendOrQueueCapture', () => {
  it('returns synced when sendCapture succeeds', async () => {
    vi.mocked(sendCapture).mockResolvedValue({ path: '/notes/example.md' });

    const result = await sendOrQueueCapture(makePayload());

    expect(result).toEqual({ state: 'synced', path: '/notes/example.md' });
    expect(sendCapture).toHaveBeenCalledTimes(1);
    expect(storage.set).not.toHaveBeenCalled();
  });

  it('queues and returns unsynced when sendCapture throws a network error', async () => {
    vi.mocked(sendCapture).mockRejectedValue(new Error('fetch failed'));

    const result = await sendOrQueueCapture(makePayload());

    expect(result.state).toBe('unsynced');
    expect(result.queued).toBe(true);
    expect(result.error).toBeTruthy();

    expect(storage.set).toHaveBeenCalledTimes(1);
    const queue = (await listQueuedCaptures());
    expect(queue).toHaveLength(1);
    expect(queue[0].state).toBe('unsynced');
    expect(queue[0].attempts).toBe(1);
    expect(queue[0].payload.url).toBe('https://example.com/article');
  });

  it('returns failed for non-recoverable errors without queuing', async () => {
    vi.mocked(sendCapture).mockRejectedValue(new ApiError('Bad Request', 400));

    const result = await sendOrQueueCapture(makePayload());

    expect(result).toEqual({ state: 'failed', error: 'Bad Request', queued: false });
    expect(storage.set).not.toHaveBeenCalled();
  });

  it('does not queue auth errors because they are recoverable', async () => {
    vi.mocked(sendCapture).mockRejectedValue(new ApiError('Unauthorized', 401));

    const result = await sendOrQueueCapture(makePayload());

    expect(result.state).toBe('unsynced');
    expect(result.queued).toBe(true);
  });
});

describe('retryQueuedCaptures', () => {
  it('syncs queued items and leaves failed ones', async () => {
    const payload1 = makePayload('https://example.com/one');
    const payload2 = makePayload('https://example.com/two');

    // Seed the queue with one unsynced item and one already-failed item.
    store[QUEUE_KEY] = [
      {
        id: 'id-1',
        payload: payload1,
        state: 'unsynced',
        attempts: 1,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
      {
        id: 'id-2',
        payload: payload2,
        state: 'failed',
        attempts: 5,
        lastError: 'too many retries',
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
    ];

    vi.mocked(sendCapture)
      .mockResolvedValueOnce({ path: '/notes/one.md' })
      .mockRejectedValueOnce(new Error('still offline'));

    const synced = await retryQueuedCaptures();

    expect(synced).toBe(1);
    const queue = await listQueuedCaptures();
    expect(queue).toHaveLength(2);
    expect(queue[0].state).toBe('synced');
    expect(queue[1].state).toBe('failed');
    expect(queue[1].attempts).toBe(6);

    const pending = await getPendingCount();
    expect(pending).toBe(1);
  });

  it('returns 0 and leaves the queue empty when there is nothing to retry', async () => {
    const synced = await retryQueuedCaptures();
    expect(synced).toBe(0);
    expect(await listQueuedCaptures()).toHaveLength(0);
  });
});

describe('getPendingCount', () => {
  it('returns the count of unsynced and failed items', async () => {
    store[QUEUE_KEY] = [
      { id: 'a', payload: makePayload(), state: 'unsynced', attempts: 1, createdAt: '', updatedAt: '' },
      { id: 'b', payload: makePayload(), state: 'synced', attempts: 1, createdAt: '', updatedAt: '' },
      { id: 'c', payload: makePayload(), state: 'failed', attempts: 5, createdAt: '', updatedAt: '' },
    ];

    expect(await getPendingCount()).toBe(2);
  });
});

describe('removeQueuedCapture', () => {
  it('clears synced items when called without an id', async () => {
    store[QUEUE_KEY] = [
      { id: 'a', payload: makePayload(), state: 'synced', attempts: 1, createdAt: '', updatedAt: '' },
      { id: 'b', payload: makePayload(), state: 'failed', attempts: 1, createdAt: '', updatedAt: '' },
    ];

    await removeQueuedCapture();

    const queue = await listQueuedCaptures();
    expect(queue).toHaveLength(1);
    expect(queue[0].id).toBe('b');
  });

  it('removes a specific capture by id', async () => {
    store[QUEUE_KEY] = [
      { id: 'a', payload: makePayload(), state: 'unsynced', attempts: 1, createdAt: '', updatedAt: '' },
      { id: 'b', payload: makePayload(), state: 'unsynced', attempts: 1, createdAt: '', updatedAt: '' },
    ];

    await removeQueuedCapture('a');

    const queue = await listQueuedCaptures();
    expect(queue).toHaveLength(1);
    expect(queue[0].id).toBe('b');
  });
});
