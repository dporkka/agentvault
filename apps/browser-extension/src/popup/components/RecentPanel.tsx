import { useState, useEffect } from 'react';
import { getRecent } from '@shared/api';
import type { SearchResult } from '@shared/types';
import { NoteViewer } from './NoteViewer';

interface RecentPanelProps {
  limit?: number;
}

function formatDate(iso: string): string {
  try {
    return new Date(iso).toLocaleString(undefined, {
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  } catch {
    return iso;
  }
}

export function RecentPanel({ limit = 20 }: RecentPanelProps) {
  const [notes, setNotes] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const loadRecent = async () => {
    setLoading(true);
    setError('');
    try {
      setNotes(await getRecent({ limit }));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load recent notes');
      setNotes([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadRecent();
  }, [limit]);

  if (selectedId) {
    return <NoteViewer id={selectedId} onBack={() => setSelectedId(null)} />;
  }

  return (
    <div className="recent-panel">
      <div className="recent-panel__header">
        <span className="recent-panel__title">Recent Notes</span>
        <button
          onClick={loadRecent}
          disabled={loading}
          className="btn btn-sm btn-secondary"
        >
          {loading ? '...' : 'Refresh'}
        </button>
      </div>
      {error && (
        <div className="banner banner-error">
          {error}
        </div>
      )}
      {!loading && notes.length === 0 && !error && (
        <div className="empty-state">
          No recent notes found.
        </div>
      )}
      {notes.length > 0 && (
        <div className="result-list">
          {notes.map((note) => (
            <button
              key={note.id}
              onClick={() => setSelectedId(note.id)}
              className="recent-card"
            >
              <div className="result-card__header">
                <span className="result-card__title">{note.title}</span>
                <span className="badge">{note.type}</span>
              </div>
              {note.snippet && (
                <p className="result-card__snippet">
                  {note.snippet}
                </p>
              )}
              <div className="recent-card__footer">
                <span>{note.project || '—'}</span>
                <span>{formatDate(note.updatedAt)}</span>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
