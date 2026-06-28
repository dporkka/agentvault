import { useState, useEffect } from 'react';
import { getNote } from '@shared/api';
import type { NoteDetail } from '@shared/types';

interface NoteViewerProps {
  id: string;
  onBack: () => void;
}

export function NoteViewer({ id, onBack }: NoteViewerProps) {
  const [note, setNote] = useState<NoteDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError('');
    getNote(id)
      .then((data: NoteDetail | null) => {
        if (!cancelled) setNote(data);
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Failed to load note');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => { cancelled = true; };
  }, [id]);

  const labelStyle: React.CSSProperties = {
    display: 'block',
    fontSize: '11px',
    fontWeight: 600,
    color: '#6b7280',
    marginBottom: '4px',
    textTransform: 'uppercase',
    letterSpacing: '0.5px',
  };

  const metaBadgeStyle: React.CSSProperties = {
    fontSize: '10px',
    padding: '3px 7px',
    background: 'rgba(79,124,255,0.12)',
    color: '#4f7cff',
    borderRadius: '4px',
    textTransform: 'uppercase',
  };

  const tagStyle: React.CSSProperties = {
    fontSize: '10px',
    padding: '2px 6px',
    background: '#1a1d27',
    color: '#9ca3af',
    borderRadius: '4px',
    border: '1px solid #2a2d3a',
  };

  return (
    <div style={{ padding: '14px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
        <button
          onClick={onBack}
          style={{
            padding: '6px 10px',
            background: '#1a1d27',
            color: '#e4e6eb',
            border: '1px solid #2a2d3a',
            borderRadius: '6px',
            fontSize: '12px',
            cursor: 'pointer',
          }}
        >
          ← Back
        </button>
      </div>
      {loading && (
        <div style={{ textAlign: 'center', color: '#6b7280', fontSize: '13px', padding: '24px' }}>
          Loading note...
        </div>
      )}
      {error && (
        <div style={{ padding: '8px 12px', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: '6px', color: '#ef4444', fontSize: '12px' }}>
          {error}
        </div>
      )}
      {!loading && !error && note === null && (
        <div style={{ textAlign: 'center', color: '#6b7280', fontSize: '13px', padding: '24px' }}>
          Note not found.
        </div>
      )}
      {note && !loading && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <div style={{ fontSize: '16px', fontWeight: 700, color: '#e4e6eb', lineHeight: 1.4 }}>
            {note.title || 'Untitled'}
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '8px' }}>
            <span style={metaBadgeStyle}>{note.type}</span>
            {note.project && <span style={metaBadgeStyle}>{note.project}</span>}
            {note.status && <span style={{ ...metaBadgeStyle, background: 'rgba(34,197,94,0.12)', color: '#22c55e' }}>{note.status}</span>}
          </div>
          {note.tags.length > 0 && (
            <div>
              <label style={labelStyle}>Tags</label>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
                {note.tags.map((tag) => (
                  <span key={tag} style={tagStyle}>{tag}</span>
                ))}
              </div>
            </div>
          )}
          <div>
            <label style={labelStyle}>Path</label>
            <div style={{ fontSize: '11px', color: '#4f7cff', fontFamily: 'monospace', wordBreak: 'break-all' }}>
              {note.path}
            </div>
          </div>
          <div>
            <label style={labelStyle}>Content</label>
            <pre style={{
              width: '100%',
              boxSizing: 'border-box',
              minHeight: '160px',
              maxHeight: '360px',
              overflow: 'auto',
              margin: 0,
              padding: '10px 12px',
              background: '#14161d',
              color: '#e4e6eb',
              border: '1px solid #2a2d3a',
              borderRadius: '6px',
              fontSize: '12px',
              lineHeight: 1.6,
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            }}>
              {note.content}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}
