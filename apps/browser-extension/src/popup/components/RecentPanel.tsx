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

  const labelStyle: React.CSSProperties = {
    display: 'block',
    fontSize: '11px',
    fontWeight: 600,
    color: '#6b7280',
    marginBottom: '4px',
    textTransform: 'uppercase',
    letterSpacing: '0.5px',
  };

  return (
    <div style={{ padding: '14px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <label style={labelStyle}>Recent Notes</label>
        <button
          onClick={loadRecent}
          disabled={loading}
          style={{
            padding: '4px 10px',
            background: '#1a1d27',
            color: '#6b7280',
            border: '1px solid #2a2d3a',
            borderRadius: '6px',
            fontSize: '11px',
            cursor: loading ? 'wait' : 'pointer',
          }}
        >
          {loading ? '...' : 'Refresh'}
        </button>
      </div>
      {error && (
        <div style={{ padding: '8px 12px', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: '6px', color: '#ef4444', fontSize: '12px' }}>
          {error}
        </div>
      )}
      {!loading && notes.length === 0 && !error && (
        <div style={{ textAlign: 'center', color: '#6b7280', fontSize: '13px', padding: '24px' }}>
          No recent notes found.
        </div>
      )}
      {notes.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {notes.map((note) => (
            <button
              key={note.id}
              onClick={() => setSelectedId(note.id)}
              style={{
                padding: '10px 12px',
                background: '#1a1d27',
                borderRadius: '6px',
                border: '1px solid #2a2d3a',
                textAlign: 'left',
                cursor: 'pointer',
                display: 'flex',
                flexDirection: 'column',
                gap: '4px',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '8px' }}>
                <span style={{ fontSize: '13px', fontWeight: 600, color: '#e4e6eb', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}>
                  {note.title}
                </span>
                <span style={{ fontSize: '10px', padding: '2px 6px', background: 'rgba(79,124,255,0.15)', color: '#4f7cff', borderRadius: '4px', textTransform: 'uppercase', flexShrink: 0 }}>
                  {note.type}
                </span>
              </div>
              {note.snippet && (
                <p style={{
                  fontSize: '12px',
                  color: '#6b7280',
                  lineHeight: '1.4',
                  overflow: 'hidden',
                  display: '-webkit-box',
                  WebkitLineClamp: 2,
                  WebkitBoxOrient: 'vertical',
                  margin: 0,
                }}>
                  {note.snippet}
                </p>
              )}
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: '11px', color: '#6b7280', marginTop: '2px' }}>
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
