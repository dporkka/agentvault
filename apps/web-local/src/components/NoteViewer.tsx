import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import ReactMarkdown from 'react-markdown';
import { EmptyState, TypeBadge } from '@agentvault/ui';
import type { NoteDetail } from '@agentvault/contract';
import { useApi } from '../hooks/useApi';
import { api } from '../api/client';
import { typeBadgeClass } from '@/utils/styles';

interface NoteViewerProps {
  note: NoteDetail;
}

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

        {/* Linked References */}
        <div className="mt-8 border-t border-vault-border pt-6">
          <h3 className="text-sm font-semibold text-vault-text-secondary mb-4">Linked References</h3>
          <LinkedReferences noteId={note.id} />
        </div>
      </div>
    </div>
  );
};

export default NoteViewer;

interface LinkedReferencesProps {
  noteId: string;
}

const LinkedReferences: React.FC<LinkedReferencesProps> = ({ noteId }) => {
  const navigate = useNavigate();
  const { data: links, loading } = useApi(
    () => api.getNoteLinks(noteId),
    [noteId]
  );

  if (loading) {
    return (
      <div className="flex justify-center py-4">
        <svg className="animate-spin h-5 w-5 text-vault-text-muted" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
          <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
          <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z" />
        </svg>
      </div>
    );
  }

  const hasBacklinks = links && links.backlinks.length > 0;
  const hasOutgoing = links && links.outgoing.length > 0;

  if (!hasBacklinks && !hasOutgoing) {
    return (
      <EmptyState
        title="No linked references"
        subtitle="Other notes that link to or from this note will appear here."
      />
    );
  }

  return (
    <div className="space-y-5">
      {hasBacklinks && (
        <div>
          <h4 className="text-xs font-semibold text-vault-text-muted uppercase tracking-wider mb-2">
            Backlinks
          </h4>
          <div className="space-y-1.5">
            {links!.backlinks.map((link) => (
              <button
                key={link.id}
                onClick={() => navigate(`/note/${link.fromNoteId}`)}
                className="w-full text-left p-3 rounded-lg border border-vault-border bg-vault-bg-secondary hover:bg-vault-bg-hover transition-colors"
              >
                <div className="flex items-center justify-between gap-2 min-w-0">
                  <span className="text-sm text-vault-text-primary truncate">
                    {link.rawTarget || link.fromNoteId}
                  </span>
                  <TypeBadge type={link.linkType} />
                </div>
              </button>
            ))}
          </div>
        </div>
      )}
      {hasOutgoing && (
        <div>
          <h4 className="text-xs font-semibold text-vault-text-muted uppercase tracking-wider mb-2">
            Outgoing Links
          </h4>
          <div className="space-y-1.5">
            {links!.outgoing.map((link) => (
              <button
                key={link.id}
                onClick={() => link.toNoteId && navigate(`/note/${link.toNoteId}`)}
                disabled={!link.toNoteId}
                className={`w-full text-left p-3 rounded-lg border border-vault-border bg-vault-bg-secondary transition-colors ${
                  link.toNoteId
                    ? 'hover:bg-vault-bg-hover cursor-pointer'
                    : 'opacity-50 cursor-default'
                }`}
              >
                <div className="flex items-center justify-between gap-2 min-w-0">
                  <span className="text-sm text-vault-text-primary truncate">
                    {link.rawTarget || link.toNoteId || 'Unresolved'}
                  </span>
                  <TypeBadge type={link.linkType} />
                </div>
              </button>
            ))}
          </div>
        </div>
      )}
    </div>
  );
};

