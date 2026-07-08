import { useState, useEffect, useCallback } from 'react';
import {
  listQueuedCaptures,
  retryQueuedCaptures,
  removeQueuedCapture,
  type QueuedCapture,
  type CaptureSyncState,
} from '@shared/capture-queue';

const STATE_COLORS: Record<CaptureSyncState, string> = {
  synced: '#22c55e',
  syncing: '#4f7cff',
  failed: '#ef4444',
  unsynced: '#f59e0b',
};

const STATE_LABELS: Record<CaptureSyncState, string> = {
  synced: 'Synced',
  syncing: 'Syncing…',
  failed: 'Failed',
  unsynced: 'Queued',
};

export function QueuePanel() {
  const [items, setItems] = useState<QueuedCapture[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    const queue = await listQueuedCaptures();
    setItems(queue.sort((a, b) => new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime()));
  }, []);

  useEffect(() => {
    refresh();
  }, [refresh]);

  const handleRetryAll = useCallback(async () => {
    setLoading(true);
    try {
      await retryQueuedCaptures();
    } finally {
      await refresh();
      setLoading(false);
    }
  }, [refresh]);

  const handleRetryOne = useCallback(
    async (id: string) => {
      setLoading(true);
      try {
        await retryQueuedCaptures();
      } finally {
        await refresh();
        setLoading(false);
      }
    },
    [refresh]
  );

  const handleRemove = useCallback(
    async (id: string) => {
      await removeQueuedCapture(id);
      await refresh();
    },
    [refresh]
  );

  const handleClearSynced = useCallback(async () => {
    await removeQueuedCapture();
    await refresh();
  }, [refresh]);

  const pending = items.filter((i) => i.state === 'unsynced' || i.state === 'failed');

  return (
    <div style={{ padding: '14px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <span style={{ fontSize: '13px', fontWeight: 600, color: '#e4e6eb' }}>
          {pending.length} pending
        </span>
        <div style={{ display: 'flex', gap: '8px' }}>
          <button
            onClick={handleRetryAll}
            disabled={loading || pending.length === 0}
            style={{
              padding: '6px 12px',
              background: 'transparent',
              border: '1px solid #4f7cff',
              borderRadius: '6px',
              color: '#4f7cff',
              fontSize: '12px',
              cursor: loading || pending.length === 0 ? 'not-allowed' : 'pointer',
              opacity: loading || pending.length === 0 ? 0.6 : 1,
            }}
          >
            Retry all
          </button>
          <button
            onClick={handleClearSynced}
            disabled={loading || items.every((i) => i.state !== 'synced')}
            style={{
              padding: '6px 12px',
              background: 'transparent',
              border: '1px solid #6b7280',
              borderRadius: '6px',
              color: '#6b7280',
              fontSize: '12px',
              cursor: loading || items.every((i) => i.state !== 'synced') ? 'not-allowed' : 'pointer',
            }}
          >
            Clear synced
          </button>
        </div>
      </div>

      {items.length === 0 && (
        <div style={{ textAlign: 'center', padding: '24px 0', color: '#6b7280', fontSize: '13px' }}>
          No queued captures.
        </div>
      )}

      {items.map((item) => (
        <div
          key={item.id}
          style={{
            padding: '10px 12px',
            background: '#14161d',
            border: '1px solid #2a2d3a',
            borderRadius: '8px',
            display: 'flex',
            flexDirection: 'column',
            gap: '6px',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px' }}>
            <span
              style={{
                color: '#e4e6eb',
                fontSize: '13px',
                fontWeight: 600,
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
              title={item.payload.title}
            >
              {item.payload.title || 'Untitled'}
            </span>
            <span
              style={{
                color: STATE_COLORS[item.state],
                fontSize: '11px',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.5px',
                whiteSpace: 'nowrap',
              }}
            >
              {STATE_LABELS[item.state]}
            </span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '10px', color: '#6b7280', fontSize: '11px' }}>
            <span>{new Date(item.createdAt).toLocaleString()}</span>
            {item.attempts > 0 && <span>Attempts: {item.attempts}</span>}
          </div>

          {item.lastError && (
            <div style={{ color: '#ef4444', fontSize: '11px', lineHeight: '1.4' }}>{item.lastError}</div>
          )}

          <div style={{ display: 'flex', gap: '8px', marginTop: '2px' }}>
            {(item.state === 'unsynced' || item.state === 'failed') && (
              <button
                onClick={() => handleRetryOne(item.id)}
                disabled={loading}
                style={{
                  padding: '4px 10px',
                  background: 'transparent',
                  border: '1px solid #4f7cff',
                  borderRadius: '4px',
                  color: '#4f7cff',
                  fontSize: '11px',
                  cursor: loading ? 'not-allowed' : 'pointer',
                  opacity: loading ? 0.6 : 1,
                }}
              >
                Retry
              </button>
            )}
            <button
              onClick={() => handleRemove(item.id)}
              disabled={loading}
              style={{
                padding: '4px 10px',
                background: 'transparent',
                border: '1px solid #ef4444',
                borderRadius: '4px',
                color: '#ef4444',
                fontSize: '11px',
                cursor: loading ? 'not-allowed' : 'pointer',
                opacity: loading ? 0.6 : 1,
              }}
            >
              Delete
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
