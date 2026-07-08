import { formatSyncResult, syncCaptures, isSyncable, canRetry } from '../sync';
import { sendCapture } from '../../api/agentvault';
import { getUnsyncedCaptures, markAsSyncing, markAsSynced, markAsFailed } from '../localInbox';
import type { Capture } from '../../types';

jest.mock('../../api/agentvault', () => ({
  sendCapture: jest.fn(),
}));

jest.mock('../localInbox', () => ({
  getUnsyncedCaptures: jest.fn(),
  markAsSyncing: jest.fn(),
  markAsSynced: jest.fn(),
  markAsFailed: jest.fn(),
}));

describe('formatSyncResult', () => {
  it('reports a single synced capture', () => {
    expect(formatSyncResult({ sent: 1, failed: 0, skipped: 0, errors: [] })).toBe(
      'Synced 1 capture',
    );
  });

  it('reports multiple synced captures', () => {
    expect(formatSyncResult({ sent: 3, failed: 0, skipped: 0, errors: [] })).toBe(
      'Synced 3 captures',
    );
  });

  it('reports mixed success and failure', () => {
    expect(formatSyncResult({ sent: 2, failed: 1, skipped: 0, errors: [] })).toBe(
      'Synced 2; 1 failed',
    );
  });

  it('reports only failures', () => {
    expect(formatSyncResult({ sent: 0, failed: 2, skipped: 0, errors: [] })).toBe(
      'Sync failed for 2 captures',
    );
  });

  it('reports skipped captures backing off', () => {
    expect(formatSyncResult({ sent: 0, failed: 0, skipped: 1, errors: [] })).toBe(
      '1 capture skipped (backing off)',
    );
  });

  it('reports nothing to sync', () => {
    expect(formatSyncResult({ sent: 0, failed: 0, skipped: 0, errors: [] })).toBe(
      'Nothing to sync',
    );
  });
});

const baseCapture: Capture = {
  id: 'cap-1',
  type: 'text',
  title: 'Note',
  text: 'body',
  project: 'inbox',
  tags: [],
  createdAt: new Date().toISOString(),
  synced: false,
  syncStatus: 'unsynced',
  retryCount: 0,
};

describe('syncCaptures', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('syncs all unsynced captures', async () => {
    (getUnsyncedCaptures as jest.Mock).mockResolvedValue([baseCapture]);
    (sendCapture as jest.Mock).mockResolvedValue(undefined);

    const result = await syncCaptures();

    expect(markAsSyncing).toHaveBeenCalledWith('cap-1');
    expect(sendCapture).toHaveBeenCalledWith({
      type: 'text',
      title: 'Note',
      text: 'body',
      project: 'inbox',
      tags: [],
    });
    expect(markAsSynced).toHaveBeenCalledWith('cap-1');
    expect(result).toEqual({ sent: 1, failed: 0, skipped: 0, errors: [] });
  });

  it('filters by captureId when provided', async () => {
    const other: Capture = { ...baseCapture, id: 'cap-2' };
    (getUnsyncedCaptures as jest.Mock).mockResolvedValue([baseCapture, other]);
    (sendCapture as jest.Mock).mockResolvedValue(undefined);

    const result = await syncCaptures({ captureId: 'cap-2' });

    expect(sendCapture).toHaveBeenCalledTimes(1);
    expect(sendCapture).toHaveBeenCalledWith(expect.objectContaining({ title: 'Note' }));
    expect(markAsSynced).toHaveBeenCalledWith('cap-2');
    expect(result.sent).toBe(1);
  });

  it('records failures and continues by default', async () => {
    (getUnsyncedCaptures as jest.Mock).mockResolvedValue([
      baseCapture,
      { ...baseCapture, id: 'cap-2' },
    ]);
    (sendCapture as jest.Mock)
      .mockRejectedValueOnce(new Error('first failed'))
      .mockResolvedValueOnce(undefined);

    const result = await syncCaptures();

    expect(markAsFailed).toHaveBeenCalledWith('cap-1', 'first failed');
    expect(markAsSynced).toHaveBeenCalledWith('cap-2');
    expect(result).toEqual({
      sent: 1,
      failed: 1,
      skipped: 0,
      errors: [{ id: 'cap-1', title: 'Note', error: 'first failed' }],
    });
  });

  it('stops after the first failure when continueOnError is false', async () => {
    (getUnsyncedCaptures as jest.Mock).mockResolvedValue([
      baseCapture,
      { ...baseCapture, id: 'cap-2' },
    ]);
    (sendCapture as jest.Mock).mockRejectedValue(new Error('boom'));

    const result = await syncCaptures({ continueOnError: false });

    expect(sendCapture).toHaveBeenCalledTimes(1);
    expect(markAsFailed).toHaveBeenCalledTimes(1);
    expect(result.failed).toBe(1);
  });

  it('skips captures that are still backing off', async () => {
    const now = Date.now();
    jest.spyOn(Date, 'now').mockReturnValue(now);
    const backingOff: Capture = {
      ...baseCapture,
      retryCount: 1,
      lastRetryAt: new Date(now - 1000).toISOString(),
    };
    (getUnsyncedCaptures as jest.Mock).mockResolvedValue([backingOff]);

    const result = await syncCaptures();

    expect(sendCapture).not.toHaveBeenCalled();
    expect(markAsSyncing).not.toHaveBeenCalled();
    expect(result).toEqual({ sent: 0, failed: 0, skipped: 1, errors: [] });

    jest.restoreAllMocks();
  });

  it('retries backing-off captures when force is true', async () => {
    const now = Date.now();
    jest.spyOn(Date, 'now').mockReturnValue(now);
    const backingOff: Capture = {
      ...baseCapture,
      retryCount: 1,
      lastRetryAt: new Date(now - 1000).toISOString(),
    };
    (getUnsyncedCaptures as jest.Mock).mockResolvedValue([backingOff]);
    (sendCapture as jest.Mock).mockResolvedValue(undefined);

    const result = await syncCaptures({ force: true });

    expect(sendCapture).toHaveBeenCalledTimes(1);
    expect(markAsSynced).toHaveBeenCalledWith('cap-1');
    expect(result).toEqual({ sent: 1, failed: 0, skipped: 0, errors: [] });

    jest.restoreAllMocks();
  });

  it('reports an empty inbox as nothing to sync', async () => {
    (getUnsyncedCaptures as jest.Mock).mockResolvedValue([]);

    const result = await syncCaptures();

    expect(sendCapture).not.toHaveBeenCalled();
    expect(result).toEqual({ sent: 0, failed: 0, skipped: 0, errors: [] });
  });
});

describe('isSyncable', () => {
  it('returns true for unsynced captures', () => {
    expect(isSyncable({ ...baseCapture, synced: false, syncStatus: 'unsynced' })).toBe(true);
  });

  it('returns false for already synced captures', () => {
    expect(isSyncable({ ...baseCapture, synced: true, syncStatus: 'synced' })).toBe(false);
  });

  it('returns false for captures currently syncing', () => {
    expect(isSyncable({ ...baseCapture, synced: false, syncStatus: 'syncing' })).toBe(false);
  });
});

describe('canRetry', () => {
  it('allows retries once backoff has elapsed', () => {
    const now = Date.now();
    jest.spyOn(Date, 'now').mockReturnValue(now);
    const capture: Capture = {
      ...baseCapture,
      retryCount: 1,
      lastRetryAt: new Date(now - 10 * 60 * 1000).toISOString(),
    };

    expect(canRetry(capture)).toBe(true);

    jest.restoreAllMocks();
  });

  it('denies retries while still backing off', () => {
    const now = Date.now();
    jest.spyOn(Date, 'now').mockReturnValue(now);
    const capture: Capture = {
      ...baseCapture,
      retryCount: 1,
      lastRetryAt: new Date(now - 1000).toISOString(),
    };

    expect(canRetry(capture)).toBe(false);

    jest.restoreAllMocks();
  });
});
