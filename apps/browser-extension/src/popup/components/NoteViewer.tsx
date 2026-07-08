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

  return (
    <div className="note-viewer">
      <div className="note-viewer__header">
        <button onClick={onBack} aria-label="Back to list" className="btn btn-sm btn-secondary">
          ← Back
        </button>
      </div>
      {loading && (
        <div className="empty-state">
          Loading note...
        </div>
      )}
      {error && (
        <div className="banner banner-error">
          {error}
        </div>
      )}
      {!loading && !error && note === null && (
        <div className="empty-state">
          Note not found.
        </div>
      )}
      {note && !loading && (
        <div className="note-viewer__content">
          <div className="note-viewer__title">
            {note.title || 'Untitled'}
          </div>
          <div className="meta-tags">
            <span className="badge">{note.type}</span>
            {note.project && <span className="badge">{note.project}</span>}
            {note.status && (
              <span className={`badge ${note.status === 'active' ? 'badge-success' : ''}`}>
                {note.status}
              </span>
            )}
          </div>
          {note.tags.length > 0 && (
            <div>
              <label className="note-viewer__label">Tags</label>
              <div className="meta-tags">
                {note.tags.map((tag) => (
                  <span key={tag} className="meta-tags__item">{tag}</span>
                ))}
              </div>
            </div>
          )}
          <div>
            <label className="note-viewer__label">Path</label>
            <div className="note-viewer__path">
              {note.path}
            </div>
          </div>
          <div>
            <label className="note-viewer__label">Content</label>
            <pre className="note-viewer__pre">
              {note.content}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}
