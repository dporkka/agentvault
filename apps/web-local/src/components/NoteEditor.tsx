import React, { useState } from 'react';
import { api } from '@/api/client';
import { MarkdownEditor } from '@agentvault/ui';

interface NoteEditorProps {
  onCreated?: (id: string, path: string) => void;
  onCancel?: () => void;
  /** If provided, edit existing note instead of creating */
  editNoteId?: string;
  editNoteTitle?: string;
  editNoteType?: string;
  editNoteContent?: string;
}

const NOTE_TYPES = ['note', 'decision', 'task', 'meeting', 'source'] as const;

const NoteEditor: React.FC<NoteEditorProps> = ({
  onCreated, onCancel, editNoteId, editNoteTitle, editNoteType, editNoteContent,
}) => {
  const isEdit = !!editNoteId;
  const [title, setTitle] = useState(editNoteTitle || '');
  const [type, setType] = useState<string>(editNoteType || 'note');
  const [project, setProject] = useState('');
  const [tags, setTags] = useState('');
  const [content, setContent] = useState(editNoteContent || '');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async () => {
    if (!title.trim()) return;
    setLoading(true);
    setError(null);

    try {
      if (isEdit) {
        const result = await api.updateNote(editNoteId!, {
          title: title,
          content: content || undefined,
          tags: tags ? tags.split(',').map(t => t.trim()) : undefined,
        });
        onCreated?.(result.id, result.path);
      } else {
        // Create: first create the note, then update with content
        const result = await api.createNote({
          type: type,
          title: title,
          project: project || undefined,
          tags: tags ? tags.split(',').map(t => t.trim()) : undefined,
        });
        if (content) {
          await api.updateNote(result.id, { content });
        }
        onCreated?.(result.id, result.path);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save note');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-6 py-3 border-b border-vault-border flex-shrink-0">
        <div className="flex items-center gap-4">
          <h2 className="text-lg font-semibold text-vault-text-primary">
            {isEdit ? 'Edit Note' : 'New Note'}
          </h2>
          <input
            type="text"
            value={title}
            onChange={e => setTitle(e.target.value)}
            placeholder="Note title..."
            className="bg-transparent text-lg text-vault-text-primary outline-none placeholder-vault-text-muted"
            style={{ minWidth: '200px' }}
          />
          {!isEdit && (
            <div className="flex items-center gap-1">
              {NOTE_TYPES.map(t => (
                <button
                  key={t}
                  onClick={() => setType(t)}
                  className={`px-2 py-0.5 text-xs rounded-full transition-colors ${
                    type === t ? 'bg-vault-accent/20 text-vault-accent' : 'text-vault-text-muted hover:text-vault-text-secondary'
                  }`}
                >
                  {t}
                </button>
              ))}
            </div>
          )}
        </div>
        <div className="flex items-center gap-3">
          {error && <span className="text-sm text-vault-error">{error}</span>}
          {onCancel && (
            <button onClick={onCancel} className="text-sm text-vault-text-muted hover:text-vault-text-secondary">
              Cancel
            </button>
          )}
          <button
            onClick={handleSubmit}
            disabled={loading || !title.trim()}
            className="px-4 py-1.5 text-sm rounded-md bg-vault-accent text-white disabled:opacity-50 hover:bg-vault-accent-hover transition-colors"
          >
            {loading ? 'Saving...' : isEdit ? 'Save' : 'Create'}
          </button>
        </div>
      </div>

      {/* Project + Tags row (create only) */}
      {!isEdit && (
        <div className="flex items-center gap-4 px-6 py-2 border-b border-vault-border flex-shrink-0 bg-vault-bg-tertiary/50">
          <input
            type="text"
            value={project}
            onChange={e => setProject(e.target.value)}
            placeholder="Project (optional)"
            className="bg-transparent text-sm text-vault-text-secondary outline-none placeholder-vault-text-muted w-40"
          />
          <input
            type="text"
            value={tags}
            onChange={e => setTags(e.target.value)}
            placeholder="Tags (comma-separated)"
            className="bg-transparent text-sm text-vault-text-secondary outline-none placeholder-vault-text-muted flex-1"
          />
        </div>
      )}

      {/* Editor */}
      <div className="flex-1 min-h-0">
        <MarkdownEditor
          value={content}
          onChange={setContent}
          onSave={handleSubmit}
          placeholder={isEdit ? 'Edit your note...' : '# Start writing...\n\nYour note content goes here.'}
          isDirty={content !== (editNoteContent || '')}
        />
      </div>
    </div>
  );
};

export default NoteEditor;
