import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import type { Note } from '@/api/types';

interface NoteViewerProps {
  note: Note;
}

const typeBadgeClass = (type: string): string => {
  switch (type) {
    case 'note': return 'type-badge-note';
    case 'decision': return 'type-badge-decision';
    case 'task': return 'type-badge-task';
    case 'meeting': return 'type-badge-meeting';
    case 'source': return 'type-badge-source';
    default: return 'type-badge-default';
  }
};

const NoteViewer: React.FC<NoteViewerProps> = ({ note }) => {
  const [showRaw, setShowRaw] = useState(false);
  const navigate = useNavigate();

  return (
    <div className="h-full flex flex-col animate-fade-in">
      {/* Header */}
      <div className="border-b border-vault-border px-6 py-4">
        <div className="flex items-center gap-2 mb-3">
          <button
            onClick={() => navigate('/')}
            className="flex items-center gap-1 text-sm text-vault-text-secondary hover:text-vault-text-primary transition-colors"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M15.75 19.5 8.25 12l7.5-7.5" />
            </svg>
            Back
          </button>
        </div>

        <div className="flex items-start justify-between gap-4">
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2 mb-1">
              <h1 className="text-xl font-semibold text-vault-text-primary truncate">
                {note.title}
              </h1>
              <span className={`type-badge ${typeBadgeClass(note.type)}`}>{note.type}</span>
            </div>
            <p className="text-sm text-vault-text-muted font-mono truncate">{note.path}</p>
          </div>

          <button
            onClick={() => setShowRaw((v) => !v)}
            className={`flex-shrink-0 flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg border transition-colors ${
              showRaw
                ? 'border-vault-accent bg-vault-accent-muted text-vault-accent'
                : 'border-vault-border text-vault-text-secondary hover:bg-vault-bg-hover hover:text-vault-text-primary'
            }`}
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
              {showRaw ? (
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.036 12.322a1.012 1.012 0 0 1 0-.639C3.423 7.51 7.36 4.5 12 4.5c4.638 0 8.573 3.007 9.963 7.178.07.207.07.431 0 .639C20.577 16.49 16.64 19.5 12 19.5c-4.638 0-8.573-3.007-9.963-7.178Z" />
              ) : (
                <path strokeLinecap="round" strokeLinejoin="round" d="M17.25 6.75 22.5 12l-5.25 5.25m-10.5 0L1.5 12l5.25-5.25m7.5-3-4.5 16.5" />
              )}
            </svg>
            {showRaw ? 'Rendered' : 'Raw'}
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto px-6 py-6">
        {showRaw ? (
          <pre className="text-sm text-vault-text-primary whitespace-pre-wrap font-mono leading-relaxed bg-vault-bg-tertiary rounded-lg p-4 border border-vault-border overflow-x-auto">
            {note.content}
          </pre>
        ) : (
          <div className="prose prose-invert prose-vault max-w-none">
            <ReactMarkdown>{note.content}</ReactMarkdown>
          </div>
        )}
      </div>
    </div>
  );
};

export default NoteViewer;
