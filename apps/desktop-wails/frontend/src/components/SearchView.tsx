import { useState, useEffect, useCallback, useRef } from 'react';
import { SearchIcon, X, FileText } from './Icons';
import type { SearchResult } from '../types';

interface Props {
  onOpenNote: (path: string) => void;
}

export default function SearchView({ onOpenNote }: Props) {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [typeFilter, setTypeFilter] = useState('');
  const [vectorEnabled, setVectorEnabled] = useState(false);
  const [hybridWeight, setHybridWeight] = useState(0.5);
  const [topK, setTopK] = useState(30);
  const inputRef = useRef<HTMLInputElement>(null);

  const performSearch = useCallback(async (q: string) => {
    if (!q.trim()) {
      // Load recent notes when no query
      try {
        const recent = await window.go.main.NoteService.GetRecent(20);
        setResults(recent);
      } catch (err) {
        console.error('Failed to load recent:', err);
        setResults([]);
      }
      return;
    }

    setLoading(true);
    try {
      const searchResults = await window.go.main.NoteService.Search(q, typeFilter, '', vectorEnabled, hybridWeight, topK);
      setResults(searchResults);
      setSelectedIndex(0);
    } catch (err) {
      console.error('Search failed:', err);
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, [typeFilter, vectorEnabled, hybridWeight, topK]);

  // Search on query change (debounced)
  useEffect(() => {
    const timer = setTimeout(() => performSearch(query), 200);
    return () => clearTimeout(timer);
  }, [query, performSearch]);

  // Keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      // Focus search on /
      if (e.key === '/' && document.activeElement !== inputRef.current) {
        e.preventDefault();
        inputRef.current?.focus();
      }
      // Navigate results
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelectedIndex(prev => Math.min(prev + 1, results.length - 1));
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelectedIndex(prev => Math.max(prev - 1, 0));
      }
      if (e.key === 'Enter' && results[selectedIndex]) {
        onOpenNote(results[selectedIndex].path);
      }
      // Escape clears search
      if (e.key === 'Escape') {
        setQuery('');
        inputRef.current?.blur();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [results, selectedIndex, onOpenNote]);

  const typeFilters = ['', 'note', 'decision', 'task', 'meeting', 'source'];

  const getTypeColor = (type: string) => {
    switch (type) {
      case 'decision': return 'bg-purple-500/20 text-purple-400';
      case 'task': return 'bg-blue-500/20 text-blue-400';
      case 'meeting': return 'bg-green-500/20 text-green-400';
      case 'source': return 'bg-yellow-500/20 text-yellow-400';
      default: return 'bg-gray-500/20 text-gray-400';
    }
  };

  return (
    <div className="flex flex-col h-full bg-[var(--bg-primary)]">
      {/* Search Bar */}
      <div className="px-4 py-3 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <div className="relative">
          <SearchIcon className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-[var(--text-muted)]" />
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search notes... (press / to focus)"
            className="w-full pl-10 pr-10 py-2 input"
          />
          {query && (
            <button
              onClick={() => setQuery('')}
              className="absolute right-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)] hover:text-[var(--text-primary)]"
            >
              <X className="w-4 h-4" />
            </button>
          )}
        </div>

        {/* Type Filters */}
        <div className="flex gap-1.5 mt-2">
          {typeFilters.map(t => (
            <button
              key={t}
              onClick={() => setTypeFilter(t)}
              className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
                typeFilter === t
                  ? 'bg-[var(--accent)] text-white'
                  : 'bg-[var(--bg-tertiary)] text-[var(--text-muted)] hover:bg-[var(--bg-hover)]'
              }`}
            >
              {t || 'All'}
            </button>
          ))}
        </div>

        {/* Vector / Hybrid Controls */}
        <div className="flex items-center gap-3 mt-2">
          <button
            onClick={() => setVectorEnabled(v => !v)}
            className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors ${
              vectorEnabled
                ? 'bg-[var(--accent)] text-white'
                : 'bg-[var(--bg-tertiary)] text-[var(--text-muted)] hover:bg-[var(--bg-hover)]'
            }`}
            title="Toggle semantic vector search"
          >
            Vector
          </button>

          {vectorEnabled && (
            <>
              <label className="flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
                <span>Hybrid</span>
                <input
                  type="range"
                  min={0}
                  max={1}
                  step={0.1}
                  value={hybridWeight}
                  onChange={(e) => setHybridWeight(parseFloat(e.target.value))}
                  className="w-20 accent-[var(--accent)]"
                  title="0 = FTS only, 1 = vector only"
                />
                <span className="w-8 text-right tabular-nums">{hybridWeight.toFixed(1)}</span>
              </label>

              <label className="flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
                <span>TopK</span>
                <input
                  type="number"
                  min={1}
                  max={200}
                  value={topK}
                  onChange={(e) => {
                    const n = parseInt(e.target.value || '1', 10);
                    setTopK(Number.isNaN(n) ? 1 : Math.min(200, Math.max(1, n)));
                  }}
                  className="w-14 px-1 py-0.5 rounded bg-[var(--bg-tertiary)] text-[var(--text-primary)] border border-[var(--border)] text-xs tabular-nums"
                />
              </label>
            </>
          )}
        </div>
      </div>

      {/* Results */}
      <div className="flex-1 overflow-auto">
        {loading ? (
          <div className="flex items-center justify-center h-32">
            <div className="text-sm text-[var(--text-muted)]">Searching...</div>
          </div>
        ) : results.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-64 text-[var(--text-muted)]">
            <SearchIcon className="w-10 h-10 mb-3 opacity-30" />
            <p className="text-sm">
              {query ? 'No results found' : 'Start typing to search your vault'}
            </p>
          </div>
        ) : (
          <div className="py-1">
            {!query && (
              <div className="px-4 py-2 text-xs font-medium text-[var(--text-muted)] uppercase tracking-wider">
                Recent Notes
              </div>
            )}
            {results.map((result, index) => (
              <button
                key={result.id}
                onClick={() => onOpenNote(result.path)}
                className={`w-full flex items-start gap-3 px-4 py-3 text-left transition-colors border-b border-[var(--border)]/50 ${
                  index === selectedIndex
                    ? 'bg-[var(--accent)]/10'
                    : 'hover:bg-[var(--bg-hover)]'
                }`}
              >
                <FileText className="w-4 h-4 mt-0.5 text-[var(--text-muted)] flex-shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-sm font-medium text-[var(--text-primary)] truncate">
                      {result.title}
                    </span>
                    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${getTypeColor(result.type)}`}>
                      {result.type}
                    </span>
                    {result.project && (
                      <span className="px-1.5 py-0.5 rounded text-[10px] bg-[var(--bg-tertiary)] text-[var(--text-muted)]">
                        {result.project}
                      </span>
                    )}
                  </div>
                  {result.snippet && (
                    <p className="text-xs text-[var(--text-muted)] line-clamp-2">
                      {result.snippet}
                    </p>
                  )}
                  <div className="flex items-center gap-2 mt-1 text-[10px] text-[var(--text-muted)]">
                    <span>{result.path}</span>
                    {result.updatedAt && (
                      <span>{result.updatedAt}</span>
                    )}
                  </div>
                </div>
              </button>
            ))}
          </div>
        )}
      </div>

      {/* Status Bar */}
      <div className="px-4 py-1.5 border-t border-[var(--border)] bg-[var(--bg-secondary)] text-xs text-[var(--text-muted)]">
        {results.length} {results.length === 1 ? 'result' : 'results'}
        {query && ` for "${query}"`}
      </div>
    </div>
  );
}
