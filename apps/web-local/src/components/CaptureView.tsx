import React, { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/client';

const CaptureView: React.FC = () => {
  const navigate = useNavigate();
  const [title, setTitle] = useState('');
  const [text, setText] = useState('');
  const [url, setUrl] = useState('');
  const [project, setProject] = useState('');
  const [tags, setTags] = useState('');
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!title.trim() && !text.trim()) return;

    setLoading(true);
    setError(null);
    setSuccess(null);

    try {
      const result = await api.capture({
        title: title.trim() || 'Untitled Capture',
        text: text.trim() || undefined,
        url: url.trim() || undefined,
        project: project.trim() || undefined,
        tags: tags
          .split(',')
          .map((t) => t.trim())
          .filter(Boolean),
      });
      setSuccess(`Saved to ${result.path}`);
      setTitle('');
      setText('');
      setUrl('');
      setProject('');
      setTags('');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to capture');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b border-vault-border px-6 py-4">
        <h1 className="text-lg font-semibold text-vault-text-primary">Capture</h1>
        <p className="text-sm text-vault-text-muted mt-1">
          Quick-capture an idea, link, or snippet to your vault inbox.
        </p>
      </div>

      {/* Form */}
      <div className="flex-1 overflow-y-auto px-6 py-4">
        <form onSubmit={handleSubmit} className="space-y-4 max-w-lg">
          {/* Title */}
          <div>
            <label htmlFor="cap-title" className="block text-sm font-medium text-vault-text-primary mb-1">
              Title
            </label>
            <input
              id="cap-title"
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Quick note title"
              className="w-full bg-vault-bg-secondary border border-vault-border rounded-md px-3 py-2 text-sm text-vault-text-primary placeholder-vault-text-muted focus:outline-none focus:ring-1 focus:ring-vault-accent"
            />
          </div>

          {/* Text */}
          <div>
            <label htmlFor="cap-text" className="block text-sm font-medium text-vault-text-primary mb-1">
              Content
            </label>
            <textarea
              id="cap-text"
              value={text}
              onChange={(e) => setText(e.target.value)}
              placeholder="What's on your mind?"
              rows={5}
              className="w-full bg-vault-bg-secondary border border-vault-border rounded-md px-3 py-2 text-sm text-vault-text-primary placeholder-vault-text-muted focus:outline-none focus:ring-1 focus:ring-vault-accent resize-vertical"
            />
          </div>

          {/* URL */}
          <div>
            <label htmlFor="cap-url" className="block text-sm font-medium text-vault-text-primary mb-1">
              URL (optional)
            </label>
            <input
              id="cap-url"
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://..."
              className="w-full bg-vault-bg-secondary border border-vault-border rounded-md px-3 py-2 text-sm text-vault-text-primary placeholder-vault-text-muted focus:outline-none focus:ring-1 focus:ring-vault-accent"
            />
          </div>

          {/* Project */}
          <div>
            <label htmlFor="cap-project" className="block text-sm font-medium text-vault-text-primary mb-1">
              Project (optional)
            </label>
            <input
              id="cap-project"
              type="text"
              value={project}
              onChange={(e) => setProject(e.target.value)}
              placeholder="my-project"
              className="w-full bg-vault-bg-secondary border border-vault-border rounded-md px-3 py-2 text-sm text-vault-text-primary placeholder-vault-text-muted focus:outline-none focus:ring-1 focus:ring-vault-accent"
            />
          </div>

          {/* Tags */}
          <div>
            <label htmlFor="cap-tags" className="block text-sm font-medium text-vault-text-primary mb-1">
              Tags (comma-separated)
            </label>
            <input
              id="cap-tags"
              type="text"
              value={tags}
              onChange={(e) => setTags(e.target.value)}
              placeholder="idea, todo, research"
              className="w-full bg-vault-bg-secondary border border-vault-border rounded-md px-3 py-2 text-sm text-vault-text-primary placeholder-vault-text-muted focus:outline-none focus:ring-1 focus:ring-vault-accent"
            />
          </div>

          {/* Error */}
          {error && (
            <div className="flex items-center gap-2 text-sm text-vault-error">
              <svg className="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
              </svg>
              <span>{error}</span>
            </div>
          )}

          {/* Success */}
          {success && (
            <div className="flex items-center gap-2 text-sm text-vault-success">
              <svg className="w-4 h-4 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
              <span>{success}</span>
            </div>
          )}

          {/* Actions */}
          <div className="flex items-center gap-3 pt-2">
            <button
              type="submit"
              disabled={loading || (!title.trim() && !text.trim())}
              className="px-4 py-2 bg-vault-accent text-white text-sm font-medium rounded-md hover:bg-vault-accent-hover disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              {loading ? 'Saving...' : 'Save Capture'}
            </button>
            <button
              type="button"
              onClick={() => navigate(-1)}
              className="px-4 py-2 text-sm text-vault-text-muted hover:text-vault-text-primary transition-colors"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};

export default CaptureView;
