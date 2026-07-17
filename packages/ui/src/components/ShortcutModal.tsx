import { useState, useEffect, useCallback } from 'react';

export interface ShortcutModalProps {
  isOpen?: boolean;
  onClose?: () => void;
}

interface Shortcut {
  keys: string[];
  description: string;
}

const SHORTCUTS: Shortcut[] = [
  { keys: ['Ctrl', 'K'], description: 'Open command palette' },
  { keys: ['Ctrl', 'S'], description: 'Save current note' },
  { keys: ['/'], description: 'Focus search' },
  { keys: ['Ctrl', 'Shift', 'N'], description: 'Create new note' },
  { keys: ['?'], description: 'Show keyboard shortcuts' },
  { keys: ['Esc'], description: 'Close modals / cancel' },
];

export function ShortcutModal({ isOpen: externalOpen, onClose }: ShortcutModalProps) {
  const [internalOpen, setInternalOpen] = useState(false);

  const isOpen = externalOpen ?? internalOpen;

  const close = useCallback(() => {
    if (externalOpen === undefined) setInternalOpen(false);
    onClose?.();
  }, [externalOpen, onClose]);

  const toggle = useCallback(() => {
    if (isOpen) {
      close();
    } else {
      if (externalOpen === undefined) setInternalOpen(true);
    }
  }, [isOpen, externalOpen, close]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === '?' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        const tag = (e.target as HTMLElement)?.tagName;
        if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT') return;
        e.preventDefault();
        toggle();
      }
      if (e.key === 'Escape' && isOpen) {
        e.preventDefault();
        close();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [isOpen, toggle, close]);

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center"
      style={{ backgroundColor: 'rgba(0,0,0,0.5)' }}
      onClick={close}
    >
      <div
        className="w-full max-w-sm rounded-lg shadow-2xl p-6"
        style={{
          backgroundColor: 'var(--av-bg-secondary, #1a1d27)',
          border: '1px solid var(--av-border, #2e3344)',
        }}
        onClick={e => e.stopPropagation()}
      >
        <h2
          className="text-lg font-semibold mb-4"
          style={{ color: 'var(--av-text-primary, #e4e6eb)' }}
        >
          Keyboard Shortcuts
        </h2>
        <div className="space-y-2">
          {SHORTCUTS.map((s) => (
            <div key={s.description} className="flex items-center justify-between text-sm">
              <span style={{ color: 'var(--av-text-secondary, #b0b3be)' }}>
                {s.description}
              </span>
              <span className="flex gap-1">
                {s.keys.map((k) => (
                  <kbd
                    key={k}
                    className="px-1.5 py-0.5 text-xs rounded font-mono"
                    style={{
                      backgroundColor: 'var(--av-bg-tertiary, #232632)',
                      color: 'var(--av-text-primary, #e4e6eb)',
                      border: '1px solid var(--av-border, #2e3344)',
                    }}
                  >
                    {k}
                  </kbd>
                ))}
              </span>
            </div>
          ))}
        </div>
        <p
          className="mt-4 text-xs text-center"
          style={{ color: 'var(--av-text-muted, #6b7084)' }}
        >
          Press Esc or click outside to dismiss
        </p>
      </div>
    </div>
  );
}

export default ShortcutModal;
