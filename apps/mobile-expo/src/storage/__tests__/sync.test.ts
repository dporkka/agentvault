import { formatSyncResult } from '../sync';

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
