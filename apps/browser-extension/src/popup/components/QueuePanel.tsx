import { useState, useEffect, useCallback } from 'react';
import {
  listQueuedCaptures,
  retryQueuedCaptures,
  removeQueuedCapture,
  type QueuedCapture,
  type CaptureSyncState,
} from '@shared/capture-queue';

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
    <div className="queue-panel">
      <div className="queue-panel__header">
        <span className="queue-panel__count">
          {loading && <span className="spinner spinner--inline" />}
          {pending.length} pending
        </span>
        <div className="queue-panel__actions">
          <button
            onClick={handleRetryAll}
            disabled={loading || pending.length === 0}
            aria-label="Retry all pending captures"
            className="btn btn-sm btn-secondary"
          >
            Retry all
          </button>
          <button
            onClick={handleClearSynced}
            disabled={loading || items.every((i) => i.state !== 'synced')}
            aria-label="Clear synced captures"
            className="btn btn-sm btn-ghost"
          >
            Clear synced
          </button>
        </div>
      </div>

      {items.length === 0 && (
        <div className="queue-empty">No queued captures.</div>
      )}

      {items.map((item) => (
        <div key={item.id} className="queue-item">
          <div className="queue-item__row">
            <span className="queue-item__title" title={item.payload.title}>
              {item.payload.title || 'Untitled'}
            </span>
            <span className={`queue-item__state queue-item__state--${item.state}`}>
              {STATE_LABELS[item.state]}
            </span>
          </div>

          <div className="queue-item__meta">
            <span>{new Date(item.createdAt).toLocaleString()}</span>
            {item.attempts > 0 && <span>Attempts: {item.attempts}</span>}
          </div>

          {item.lastError && (
            <div className="queue-item__error">{item.lastError}</div>
          )}

          <div className="queue-item__actions">
            {(item.state === 'unsynced' || item.state === 'failed') && (
              <button
                onClick={() => handleRetryOne(item.id)}
                disabled={loading}
                aria-label={`Retry capture ${item.payload.title || 'Untitled'}`}
                className="btn btn-sm btn-secondary"
              >
                {loading ? <span className="spinner" aria-hidden="true" /> : 'Retry'}
              </button>
            )}
            <button
              onClick={() => handleRemove(item.id)}
              disabled={loading}
              aria-label={`Delete capture ${item.payload.title || 'Untitled'}`}
              className="btn btn-sm btn-danger"
            >
              Delete
            </button>
          </div>
        </div>
      ))}
    </div>
  );
}
