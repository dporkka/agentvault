import React, { useState } from 'react';
import { api } from '@/api/client';

interface NoteEditorProps {
  onCreated?: (id: string, path: string) => void;
  onCancel?: () => void;
}

const NOTE_TYPES = ['note', 'decision', 'task', 'meeting', 'source'] as const;

const NoteEditor: React.FC<NoteEditorProps> = ({ onCreated, onCancel }) => {
  const [title, setTitle] = useState('');
  const [type, setType] = useState<(typeof NOTE_TYPES)[number]>('note');
  const [project, setProject] = useState('');
  const [tags, setTags] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim()) return;

    setLoading(true);
    setError(null);

    try {
      const result = await api.createNote({
        type,
        title: title.trim(),
        project: project.trim() || undefined,
        tags: tags.split(',').map((t) => t.trim()).filter(Boolean),
      });
      onCreated?.(result.id, result.path);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create note');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="p-6 animate-fade-in">
      <h2 className="text-lg font-semibold text-vault-text-primary mb-4">Create Note</h2>

      <form onSubmit={handleSubmit} className="space-y-4 max-w-lg">
        {/* Title */}
        <div>
          <label className="block text-sm font-medium text-vault-text-secondary mb-1">Title *</label>
          <input
            type="text"
            value={title}
            onChange={(e) => setTitle(e.target.value)}
            placeholder="Note title"
            required
            className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2 text-sm text-vault-text-primary placeholder-vault-text-muted focus:border-vault-accent focus:ring-1 focus:ring-vault-accent transition-colors outline-none"
          />
        </div>

        {/* Type */}
        <div>
          <label className="block text-sm font-medium text-vault-text-secondary mb-1">Type</label>
          <div className="flex flex-wrap gap-1.5">
            {NOTE_TYPES.map((t) => (
              <button
                key={t}
                type="button"
                onClick={() => setType(t)}
                className={`px-3 py-1 text-xs font-medium rounded-full capitalize transition-colors ${
                  type === t
                    ? 'bg-vault-accent text-white'
                    : 'bg-vault-bg-tertiary text-vault-text-secondary hover:bg-vault-bg-hover'
                }`}
              >
                {t}
              </button>
            ))}
          </div>
        </div>

        {/* Project */}
        <div>
          <label className="block text-sm font-medium text-vault-text-secondary mb-1">Project</label>
          <input
            type="text"
            value={project}
            onChange={(e) => setProject(e.target.value)}
            placeholder="Project name (optional)"
            className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2 text-sm text-vault-text-primary placeholder-vault-text-muted focus:border-vault-accent focus:ring-1 focus:ring-vault-accent transition-colors outline-none"
          />
        </div>

        {/* Tags */}
        <div>
          <label className="block text-sm font-medium text-vault-text-secondary mb-1">Tags</label>
          <input
            type="text"
            value={tags}
            onChange={(e) => setTags(e.target.value)}
            placeholder="tag1, tag2, tag3"
            className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2 text-sm text-vault-text-primary placeholder-vault-text-muted focus:border-vault-accent focus:ring-1 focus:ring-vault-accent transition-colors outline-none"
          />
          <p className="text-xs text-vault-text-muted mt-1">Comma-separated list of tags</p>
        </div>

        {/* Error */}
        {error && (
          <div className="flex items-center gap-2 text-sm text-vault-error">
            <svg className="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
            </svg>
            {error}
          </div>
        )}

        {/* Actions */}
        <div className="flex items-center gap-3 pt-2">
          <button
            type="submit"
            disabled={loading || !title.trim()}
            className="flex items-center gap-2 px-4 py-2 bg-vault-accent hover:bg-vault-accent-hover disabled:opacity-50 disabled:cursor-not-allowed text-white text-sm font-medium rounded-lg transition-colors"
          >
            {loading && <div className="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin" />}
            Create Note
          </button>
          {onCancel && (
            <button
              type="button"
              onClick={onCancel}
              className="px-4 py-2 text-sm text-vault-text-secondary hover:text-vault-text-primary transition-colors"
            >
              Cancel
            </button>
          )}
        </div>
      </form>
    </div>
  );
};

export default NoteEditor;
