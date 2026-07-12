import React from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import NoteViewer from './NoteViewer';
import { useApi } from '../hooks/useApi';
import { api } from '../api/client';

const NotePage: React.FC = () => {
  const { id } = useParams();
  const navigate = useNavigate();
  const noteId = id ?? '';
  const { data: note, loading, error } = useApi(
    () => api.getNote(noteId),
    [noteId]
  );

  if (loading) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="flex items-center gap-3 text-vault-text-muted">
          <div className="w-5 h-5 border-2 border-vault-accent border-t-transparent rounded-full animate-spin" />
          Loading note...
        </div>
      </div>
    );
  }

  if (error || !note) {
    return (
      <div className="h-full flex items-center justify-center">
        <div className="text-center">
          <svg className="w-10 h-10 text-vault-error mx-auto mb-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1}>
            <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
          </svg>
          <p className="text-sm text-vault-error mb-2">{error || 'Note not found'}</p>
          <button
            onClick={() => navigate('/')}
            className="text-sm text-vault-accent hover:underline"
          >
            Back to search
          </button>
        </div>
      </div>
    );
  }

  return <NoteViewer note={note} />;
};

export default NotePage;
