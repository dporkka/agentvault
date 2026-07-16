import React, { useState, useEffect, useCallback, useRef } from 'react';

export interface CommandPaletteProps {
  onOpen?: () => void;
  onClose?: () => void;
  onSelectNote?: (id: string) => void;
  onSelectAction?: (action: string) => void;
  searchNotes?: (query: string, limit: number) => Promise<SearchResult[]>;
  isOpen?: boolean;
  onToggle?: () => void;
}

export interface SearchResult {
  id: string;
  title: string;
  type?: string;
  path?: string;
  snippet?: string;
}

interface CommandItem {
  id: string;
  label: string;
  icon: string;
  action: string;
  category: 'navigation' | 'action';
}

interface NoteItem extends SearchResult {
  category: 'note';
  icon: string;
  label: string;
  action: string;
}

type PaletteItem = CommandItem | NoteItem;

const BUILTIN_COMMANDS: CommandItem[] = [
  { id: 'nav-search', label: 'Search vault', icon: '🔍', action: '/search', category: 'navigation' },
  { id: 'nav-dashboard', label: 'Dashboard', icon: '📊', action: '/', category: 'navigation' },
  { id: 'nav-daily', label: 'Daily note', icon: '📅', action: '/daily', category: 'navigation' },
  { id: 'nav-projects', label: 'Projects', icon: '📁', action: '/projects', category: 'navigation' },
  { id: 'nav-tags', label: 'Tags', icon: '🏷️', action: '/tags', category: 'navigation' },
  { id: 'nav-settings', label: 'Settings', icon: '⚙️', action: '/settings', category: 'navigation' },
  { id: 'act-new', label: 'Create note', icon: '✏️', action: 'new-note', category: 'action' },
  { id: 'act-capture', label: 'Quick capture', icon: '📥', action: 'capture', category: 'action' },
  { id: 'act-ask', label: 'Ask AI', icon: '✨', action: 'ask', category: 'action' },
];

export const CommandPalette: React.FC<CommandPaletteProps> = ({
  onOpen, onClose, onSelectNote, onSelectAction, searchNotes,
  isOpen: externalOpen, onToggle,
}) => {
  const [internalOpen, setInternalOpen] = useState(false);
  const [query, setQuery] = useState('');
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [noteResults, setNoteResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);
  const debounceTimer = useRef<number | undefined>(undefined);


  const isOpen = externalOpen ?? internalOpen;
  const open = useCallback(() => {
    if (externalOpen === undefined) setInternalOpen(true);
    onToggle?.();
    onOpen?.();
    setTimeout(() => inputRef.current?.focus(), 50);
  }, [externalOpen, onToggle, onOpen]);

  const close = useCallback(() => {
    if (externalOpen === undefined) setInternalOpen(false);
    onToggle?.();
    setQuery('');
    setNoteResults([]);
    setSelectedIndex(0);
    onClose?.();
  }, [externalOpen, onToggle, onClose]);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        isOpen ? close() : open();
      }
      if (e.key === 'Escape' && isOpen) {
        e.preventDefault();
        close();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [isOpen, open, close]);

  useEffect(() => {
    if (!searchNotes || query.trim().length < 2) {
      setNoteResults([]);
      return;
    }
    if (debounceTimer.current) clearTimeout(debounceTimer.current);
    debounceTimer.current = setTimeout(async () => {
      setLoading(true);
      try {
        const results = await searchNotes(query, 5);
        setNoteResults(results);
      } catch {
        setNoteResults([]);
      }
      setLoading(false);
    }, 150);
    return () => { if (debounceTimer.current) clearTimeout(debounceTimer.current); };
  }, [query, searchNotes]);

  const filteredCommands = query.trim().length > 0
    ? BUILTIN_COMMANDS.filter(c => c.label.toLowerCase().includes(query.toLowerCase()))
    : BUILTIN_COMMANDS;

  const allItems: PaletteItem[] = [
    ...filteredCommands,
    ...noteResults.map(n => ({
      ...n, category: 'note' as const, action: n.id, label: n.title, icon: '📄',
    })),
  ];

  useEffect(() => { setSelectedIndex(0); }, [query]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex(i => Math.min(i + 1, allItems.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex(i => Math.max(i - 1, 0));
    } else if (e.key === 'Enter' && allItems[selectedIndex]) {
      e.preventDefault();
      const item = allItems[selectedIndex];
      if (item.category === 'note') {
        onSelectNote?.(item.id);
      } else {
        onSelectAction?.(item.action);
      }
      close();
    }
  };

  if (!isOpen) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-[15vh]"
      style={{ backgroundColor: 'rgba(0,0,0,0.5)' }}
      onClick={close}
    >
      <div
        className="w-full max-w-lg rounded-lg shadow-2xl overflow-hidden"
        style={{
          backgroundColor: 'var(--av-bg-secondary, #1a1d27)',
          border: '1px solid var(--av-border, #2e3344)',
        }}
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center px-4 py-3 border-b" style={{ borderColor: 'var(--av-border, #2e3344)' }}>
          <span className="mr-3 text-lg opacity-50">🔍</span>
          <input ref={inputRef} type="text" value={query}
            onChange={e => setQuery(e.target.value)} onKeyDown={handleKeyDown}
            placeholder="Search notes or run a command..."
            className="flex-1 bg-transparent outline-none text-sm"
            style={{ color: 'var(--av-text-primary, #e4e6eb)' }} autoFocus />
          {loading && (
            <span className="ml-2 w-4 h-4 border-2 border-t-transparent rounded-full animate-spin"
              style={{ borderColor: 'var(--av-accent, #4f7cff)', borderTopColor: 'transparent' }} />
          )}
        </div>
        <div className="max-h-80 overflow-y-auto">
          {allItems.length === 0 && query.trim().length > 0 && (
            <div className="px-4 py-6 text-center text-sm" style={{ color: 'var(--av-text-muted)' }}>
              No results found
            </div>
          )}
          {allItems.map((item, i) => (
            <div key={item.category === 'note' ? `note-${item.id}` : item.id}
              className="flex items-center px-4 py-2.5 cursor-pointer text-sm"
              style={{
                backgroundColor: i === selectedIndex ? 'var(--av-bg-hover)' : 'transparent',
                color: 'var(--av-text-primary)',
              }}
              onMouseEnter={() => setSelectedIndex(i)}
              onClick={() => {
                item.category === 'note' ? onSelectNote?.(item.id) : onSelectAction?.(item.action);
                close();
              }}>
              <span className="mr-3 text-sm opacity-50">{item.icon}</span>
              <span className="flex-1 truncate">{item.label}</span>
              {item.category === 'note' && 'path' in item && (
                <span className="ml-2 text-xs truncate opacity-50 max-w-[40%]">{item.path}</span>
              )}
              <span className="ml-2 text-xs opacity-40">
                {item.category === 'navigation' ? 'Nav' : item.category === 'action' ? 'Act' : 'Note'}
              </span>
            </div>
          ))}
        </div>
        <div className="px-4 py-2 text-xs flex gap-4 border-t"
          style={{ borderColor: 'var(--av-border)', color: 'var(--av-text-muted)' }}>
          <span>↑↓ nav</span><span>↵ select</span><span>esc close</span>
        </div>
      </div>
    </div>
  );
};

export default CommandPalette;
