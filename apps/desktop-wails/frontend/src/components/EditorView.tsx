import { useState, useEffect, useCallback } from 'react';
import CodeMirror from '@uiw/react-codemirror';
import type { Extension } from '@codemirror/state';
import { oneDark } from '@codemirror/theme-one-dark';
import ReactMarkdown from 'react-markdown';
import { Save, PanelRight, PanelRightClose, FileText, Loader2 } from './Icons';

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
  const [noteType, setNoteType] = useState<string>('note');
  const [isDirty, setIsDirty] = useState(false);
  const [showPreview, setShowPreview] = useState(true);
  // The path the editor actually writes to. For an existing note this mirrors
  // the prop; for a new note it is assigned a unique path on first save so
  // repeated saves update one file instead of overwriting a fixed name.
  const [savePath, setSavePath] = useState<string | null>(notePath);
  const [markdownExt, setMarkdownExt] = useState<Extension | null>(null);

  // Lazy-load the markdown language support so it becomes a separate chunk
  // and does not bloat the core CodeMirror bundle.
  useEffect(() => {
    let cancelled = false;
    import('@codemirror/lang-markdown').then((mod) => {
      if (!cancelled) {
        setMarkdownExt(mod.markdown());
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

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
    let path = savePath;
    if (!path) {
      // For a brand-new note, route through CreateNote so the file lands
      // in the correct folder per note type (templates.FolderRelForType).
      const relPath = await window.go.main.NoteService.CreateNote(noteType, title, '');
      path = relPath;
      setSavePath(path);
    }
    await window.go.main.NoteService.SaveNote(path, content);
    setOriginalContent(content);
    setIsDirty(false);
  }, [savePath, title, content, noteType]);

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
    <div className="flex flex-col h-full bg-bg-primary">
      {/* Toolbar */}
      <div className="flex items-center justify-between px-4 py-2 border-b border-border bg-bg-secondary">
        <div className="flex items-center gap-3">
          <FileText className="w-4 h-4 text-text-muted" />
          <div className="flex flex-col gap-1">
            <span className="text-sm font-medium text-text-primary">
              {title}
            </span>
            {!savePath && (
              <div className="flex items-center gap-1">
                {(['note', 'decision', 'task', 'meeting', 'source'] as const).map((t) => (
                  <button
                    key={t}
                    onClick={() => setNoteType(t)}
                    className={`text-[10px] px-1.5 py-0.5 rounded-full transition-colors ${
                      noteType === t
                        ? 'type-badge type-badge-' + t
                        : 'bg-bg-tertiary text-text-muted hover:text-text-secondary'
                    }`}
                  >
                    {t}
                  </button>
                ))}
              </div>
            )}
            {isDirty && (
              <span className="text-xs text-warning">unsaved</span>
            )}
            {savePath && (
              <span className="text-xs text-text-muted">{savePath}</span>
            )}
          </div>
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
          {markdownExt ? (
            <CodeMirror
              value={content}
              height="100%"
              extensions={[markdownExt]}
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
          ) : (
            <div className="flex items-center justify-center h-full text-text-muted text-sm gap-2">
              <Loader2 className="w-4 h-4 animate-spin" />
              Loading editor...
            </div>
          )}
        </div>

        {showPreview && (
          <div className="w-2/5 border-l border-border overflow-auto bg-bg-secondary p-6">
            <div className="prose prose-invert prose-sm max-w-none">
              <ReactMarkdown>{content}</ReactMarkdown>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
