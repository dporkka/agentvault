import React, { useState, useCallback } from 'react';
import ReactMarkdown from 'react-markdown';

export interface MarkdownEditorProps {
  /** Current markdown content */
  value: string;
  /** Called on every change */
  onChange: (value: string) => void;
  /** Called when user triggers save (Ctrl+S or button) */
  onSave?: () => void;
  /** Placeholder text */
  placeholder?: string;
  /** Whether to show the preview pane (default: true) */
  showPreview?: boolean;
  /** Called when preview is toggled */
  onTogglePreview?: () => void;
  /** Whether content has unsaved changes */
  isDirty?: boolean;
  /** Additional className for the container */
  className?: string;
}

export const MarkdownEditor: React.FC<MarkdownEditorProps> = ({
  value,
  onChange,
  onSave,
  placeholder = 'Start writing markdown...',
  showPreview: externalPreview,
  onTogglePreview,
  isDirty,
  className = '',
}) => {
  const [internalPreview, setInternalPreview] = useState(true);
  const showPreviewPane = externalPreview ?? internalPreview;

  const togglePreview = useCallback(() => {
    if (onTogglePreview) {
      onTogglePreview();
    } else {
      setInternalPreview(p => !p);
    }
  }, [onTogglePreview]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key === 's') {
      e.preventDefault();
      onSave?.();
    }
    // Tab key inserts spaces
    if (e.key === 'Tab') {
      e.preventDefault();
      const textarea = e.currentTarget as HTMLTextAreaElement;
      const start = textarea.selectionStart;
      const end = textarea.selectionEnd;
      const newValue = value.substring(0, start) + '  ' + value.substring(end);
      onChange(newValue);
      setTimeout(() => {
        textarea.selectionStart = textarea.selectionEnd = start + 2;
      }, 0);
    }
  }, [value, onChange, onSave]);

  const insertFormatting = useCallback((before: string, after: string) => {
    const textarea = document.querySelector('.markdown-editor__textarea') as HTMLTextAreaElement;
    if (!textarea) return;
    const start = textarea.selectionStart;
    const end = textarea.selectionEnd;
    const selected = value.substring(start, end);
    const newValue = value.substring(0, start) + before + selected + after + value.substring(end);
    onChange(newValue);
    setTimeout(() => {
      textarea.focus();
      textarea.selectionStart = start + before.length;
      textarea.selectionEnd = start + before.length + selected.length;
    }, 0);
  }, [value, onChange]);

  return (
    <div className={`flex flex-col h-full ${className}`}>
      {/* Toolbar */}
      <div className="flex items-center gap-1 px-3 py-1.5 border-b flex-shrink-0" style={{
        borderColor: 'var(--av-border, #2e3344)',
        backgroundColor: 'var(--av-bg-tertiary, #232734)',
      }}>
        {[
          { label: 'B', title: 'Bold (**text**)', action: () => insertFormatting('**', '**') },
          { label: 'I', title: 'Italic (*text*)', action: () => insertFormatting('*', '*') },
          { label: 'H', title: 'Heading (## text)', action: () => {
            const ta = document.querySelector('.markdown-editor__textarea') as HTMLTextAreaElement;
            if (ta) {
              const lineStart = value.lastIndexOf('\n', ta.selectionStart - 1) + 1;
              const newValue = value.substring(0, lineStart) + '## ' + value.substring(lineStart);
              onChange(newValue);
            }
          }},
          { label: '🔗', title: 'Link ([text](url))', action: () => insertFormatting('[', '](url)') },
          { label: '•', title: 'Bullet list (- item)', action: () => {
            const ta = document.querySelector('.markdown-editor__textarea') as HTMLTextAreaElement;
            if (ta) {
              const lineStart = value.lastIndexOf('\n', ta.selectionStart - 1) + 1;
              const newValue = value.substring(0, lineStart) + '- ' + value.substring(lineStart);
              onChange(newValue);
            }
          }},
          { label: '</>', title: 'Code block', action: () => insertFormatting('```\n', '\n```') },
        ].map(btn => (
          <button
            key={btn.label}
            title={btn.title}
            onClick={btn.action}
            className="px-2 py-0.5 text-xs rounded hover:opacity-80 transition-opacity"
            style={{ color: 'var(--av-text-secondary, #9ca3af)' }}
          >
            {btn.label}
          </button>
        ))}
        <div className="flex-1" />
        {isDirty && (
          <span className="text-xs" style={{ color: 'var(--av-warning, #f59e0b)' }}>unsaved</span>
        )}
        <button
          onClick={togglePreview}
          className="px-2 py-0.5 text-xs rounded hover:opacity-80 transition-opacity"
          style={{ color: 'var(--av-text-secondary, #9ca3af)' }}
        >
          {showPreviewPane ? 'Hide Preview' : 'Show Preview'}
        </button>
        {onSave && (
          <button
            onClick={onSave}
            disabled={!isDirty}
            className="px-3 py-0.5 text-xs rounded font-medium disabled:opacity-40 transition-opacity"
            style={{
              backgroundColor: 'var(--av-accent, #4f7cff)',
              color: '#fff',
            }}
          >
            Save
          </button>
        )}
      </div>

      {/* Editor + Preview */}
      <div className="flex flex-1 min-h-0">
        {/* Textarea */}
        <textarea
          className="markdown-editor__textarea flex-1 resize-none p-4 font-mono text-sm leading-relaxed outline-none"
          style={{
            backgroundColor: 'var(--av-bg-primary, #0f1117)',
            color: 'var(--av-text-primary, #e4e6eb)',
            border: 'none',
            width: showPreviewPane ? '50%' : '100%',
          }}
          value={value}
          onChange={e => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder={placeholder}
          spellCheck={false}
        />

        {/* Preview */}
        {showPreviewPane && (
          <div
            className="flex-1 overflow-y-auto p-4 border-l prose prose-sm prose-invert max-w-none"
            style={{
              borderColor: 'var(--av-border, #2e3344)',
              backgroundColor: 'var(--av-bg-secondary, #1a1d27)',
              color: 'var(--av-text-primary, #e4e6eb)',
            }}
          >
            {value.trim() ? (
              <ReactMarkdown>{value}</ReactMarkdown>
            ) : (
              <p style={{ color: 'var(--av-text-muted, #6b7280)' }}>Preview will appear here...</p>
            )}
          </div>
        )}
      </div>
    </div>
  );
};

export default MarkdownEditor;
