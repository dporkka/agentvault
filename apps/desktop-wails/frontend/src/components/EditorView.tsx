import { useState, useEffect, useCallback } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import { markdown } from '@codemirror/lang-markdown';
import { oneDark } from '@codemirror/theme-one-dark';
import ReactMarkdown from 'react-markdown';
import { Save, PanelRight, PanelRightClose, FileText } from './Icons';

interface Props {
  notePath: string | null;
  vaultPath: string;
  aiPanelOpen: boolean;
  onToggleAIPanel: () => void;
}

export default function EditorView({ notePath, aiPanelOpen, onToggleAIPanel }: Props) {
  const [content, setContent] = useState('');
  const [originalContent, setOriginalContent] = useState('');
  const [title, setTitle] = useState('Untitled');
  const [isDirty, setIsDirty] = useState(false);
  const [showPreview, setShowPreview] = useState(true);
  // The path the editor actually writes to. For an existing note this mirrors
  // the prop; for a new note it is assigned a unique path on first save so
  // repeated saves update one file instead of overwriting a fixed name.
  const [savePath, setSavePath] = useState<string | null>(notePath);

  useEffect(() => {
    setSavePath(notePath);
    if (notePath) {
      window.go.main.NoteService.GetNoteContent(notePath)
        .then(data => {
          setContent(data);
          setOriginalContent(data);
          // Extract title from first H1 or frontmatter
          const match = data.match(/^# (.+)$/m) || data.match(/title:\s*(.+)$/m);
          setTitle(match ? match[1] : notePath.split('/').pop() || 'Untitled');
          setIsDirty(false);
        })
        .catch(err => {
          console.error('Failed to load note:', err);
          setContent('# New Note\n\nStart writing...');
          setOriginalContent('');
          setTitle('New Note');
        });
    } else {
      setContent('# New Note\n\nStart writing...');
      setOriginalContent('');
      setTitle('New Note');
      setIsDirty(false);
    }
  }, [notePath]);

  const handleChange = useCallback((value: string) => {
    setContent(value);
    setIsDirty(value !== originalContent);
  }, [originalContent]);

  const handleSave = useCallback(async () => {
    // For a brand-new note, generate a unique filename once so we don't
    // clobber a fixed "untitled.md"; reuse it for subsequent saves.
    let path = savePath;
    if (!path) {
      const slug =
        (title || 'untitled')
          .toLowerCase()
          .replace(/[^a-z0-9]+/g, '-')
          .replace(/^-+|-+$/g, '') || 'untitled';
      path = `10-notes/${slug}-${Date.now()}.md`;
      setSavePath(path);
    }
    await window.go.main.NoteService.SaveNote(path, content);
    setOriginalContent(content);
    setIsDirty(false);
  }, [savePath, title, content]);

  // Keyboard shortcut: Ctrl+S
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 's') {
        e.preventDefault();
        handleSave();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [handleSave]);

  return (
    <div className="flex flex-col h-full bg-[var(--bg-primary)]">
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <div className="flex items-center gap-3">
          <FileText className="w-4 h-4 text-[var(--text-muted)]" />
          <span className="text-sm font-medium text-[var(--text-primary)]">
            {title}
          </span>
          {isDirty && (
            <span className="text-xs text-[var(--warning)]">unsaved</span>
          )}
          {savePath && (
            <span className="text-xs text-[var(--text-muted)] ml-2">{savePath}</span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowPreview(!showPreview)}
            className="btn-ghost text-xs"
          >
            {showPreview ? 'Hide Preview' : 'Show Preview'}
          </button>
          <button
            onClick={handleSave}
            disabled={!isDirty}
            className="btn-primary flex items-center gap-1.5 disabled:opacity-50"
          >
            <Save className="w-3.5 h-3.5" />
            Save
          </button>
          <button
            onClick={onToggleAIPanel}
            className="btn-ghost"
            title={aiPanelOpen ? 'Close AI Panel' : 'Open AI Panel'}
          >
            {aiPanelOpen ? (
              <PanelRightClose className="w-4 h-4" />
            ) : (
              <PanelRight className="w-4 h-4" />
            )}
          </button>
        </div>
      </div>

      {/* Editor + Preview */}
      <div className="flex flex-1 overflow-hidden">
        <div className={`${showPreview ? 'w-3/5' : 'w-full'} overflow-auto`}>
          <CodeMirror
            value={content}
            height="100%"
            extensions={[markdown()]}
            theme={oneDark}
            onChange={handleChange}
            basicSetup={{
              lineNumbers: true,
              highlightActiveLineGutter: true,
              highlightActiveLine: true,
              foldGutter: false,
            }}
            className="h-full text-sm"
          />
        </div>

        {showPreview && (
          <div className="w-2/5 border-l border-[var(--border)] overflow-auto bg-[var(--bg-secondary)] p-6">
            <div className="prose prose-invert prose-sm max-w-none">
              <ReactMarkdown>{content}</ReactMarkdown>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
