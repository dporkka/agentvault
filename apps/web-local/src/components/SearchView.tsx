import React, { useState, useEffect, useRef, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/client';
import { useDebounce } from '@/hooks/useDebounce';
import EmptyState from '@/components/EmptyState';
import { typeBadgeClass, TYPE_FILTERS, type TypeFilter } from '@/utils/styles';

import type { SearchResult, SearchParams } from '@agentvault/contract';
const SearchView: React.FC = () => {
  const [query, setQuery] = useState('');
  const [typeFilter, setTypeFilter] = useState<TypeFilter>('all');
  const [vectorEnabled, setVectorEnabled] = useState(false);
  const [hybridWeight, setHybridWeight] = useState(0.5);
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [selectedIndex, setSelectedIndex] = useState(-1);
  const searchInputRef = useRef<HTMLInputElement>(null);
  const resultsRef = useRef<HTMLDivElement>(null);
  const navigate = useNavigate();

  const debouncedQuery = useDebounce(query, 200);

  useEffect(() => {
    if (debouncedQuery.trim().length === 0) {
      setResults([]);
      setSelectedIndex(-1);
      return;
    }

    const controller = new AbortController();

    async function doSearch() {
      try {
        setLoading(true);
        setError(null);
        const type = typeFilter === 'all' ? undefined : typeFilter;
        const params: SearchParams = {
          q: debouncedQuery,
          type,
          vector: vectorEnabled || undefined,
          hybridWeight: vectorEnabled ? hybridWeight : undefined,
        };
        const res = await api.search(params, controller.signal);
        setResults(res);
        setSelectedIndex(-1);
      } catch (err) {
        if (err instanceof DOMException && err.name === 'AbortError') return;
        setError(err instanceof Error ? err.message : 'Search failed');
      } finally {
        setLoading(false);
      }
    }

    doSearch();

    return () => controller.abort();
  }, [debouncedQuery, typeFilter, vectorEnabled, hybridWeight]);

  // Keyboard shortcuts
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setSelectedIndex((prev) => Math.min(prev + 1, results.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setSelectedIndex((prev) => Math.max(prev - 1, -1));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      if (selectedIndex >= 0 && results[selectedIndex]) {
        navigate(`/note/${encodeURIComponent(results[selectedIndex].id)}`);
      }
    } else if (e.key === 'Escape') {
      setQuery('');
      searchInputRef.current?.blur();
    }
  }, [results, selectedIndex, navigate]);
  // Global "/" shortcut
  useEffect(() => {
    function onDocKeyDown(e: KeyboardEvent) {
      if (e.key === '/' && document.activeElement !== searchInputRef.current) {
        e.preventDefault();
        searchInputRef.current?.focus();
      }
    }
    document.addEventListener('keydown', onDocKeyDown);
    return () => document.removeEventListener('keydown', onDocKeyDown);
  }, []);

  // Scroll selected into view
  useEffect(() => {
    if (selectedIndex >= 0 && resultsRef.current) {
      const el = resultsRef.current.children[selectedIndex] as HTMLElement;
      el?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
    }
  }, [selectedIndex]);

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b border-vault-border px-6 py-4">
        <h1 className="text-lg font-semibold text-vault-text-primary mb-3">Search</h1>

        {/* Search input */}
        <div className="relative mb-3">
          <svg
            className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-vault-text-muted"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
            strokeWidth={2}
          >
            <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
          </svg>
          <input
            ref={searchInputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Search notes... (press / to focus)"
            className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg pl-10 pr-4 py-2.5 text-sm text-vault-text-primary placeholder-vault-text-muted focus:border-vault-accent focus:ring-1 focus:ring-vault-accent transition-colors outline-none"
            autoComplete="off"
            role="combobox"
            aria-expanded={results.length > 0}
            aria-controls="search-results"
            aria-activedescendant={selectedIndex >= 0 ? `result-${selectedIndex}` : undefined}
          />
          {loading && (
            <div className="absolute right-3 top-1/2 -translate-y-1/2">
              <div className="w-4 h-4 border-2 border-vault-accent border-t-transparent rounded-full animate-spin" />
            </div>
          )}
        </div>

        {/* Type filters */}
        <div className="flex flex-wrap gap-1.5">
          {TYPE_FILTERS.map((t) => (
            <button
              key={t}
              onClick={() => setTypeFilter(t)}
              className={`px-3 py-1 text-xs font-medium rounded-full capitalize transition-colors ${
                typeFilter === t
                  ? 'bg-vault-accent text-white'
                  : 'bg-vault-bg-tertiary text-vault-text-secondary hover:bg-vault-bg-hover hover:text-vault-text-primary'
              }`}
            >
              {t}
            </button>
          ))}
        </div>

        {/* Vector search toggle */}
        <div className="mt-3 flex items-center gap-4">
          <label className="flex items-center gap-2 text-xs text-vault-text-secondary cursor-pointer select-none">
            <input
              type="checkbox"
              checked={vectorEnabled}
              onChange={(e) => setVectorEnabled(e.target.checked)}
              className="rounded border-vault-border bg-vault-bg-tertiary text-vault-accent focus:ring-vault-accent"
            />
            Vector search
          </label>
          {vectorEnabled && (
            <div className="flex items-center gap-2 text-xs text-vault-text-secondary">
              <span>Hybrid weight</span>
              <input
                type="range"
                min={0}
                max={1}
                step={0.1}
                value={hybridWeight}
                onChange={(e) => setHybridWeight(parseFloat(e.target.value))}
                className="w-24 accent-vault-accent"
              />
              <span className="font-mono w-8">{hybridWeight.toFixed(1)}</span>
            </div>
          )}
        </div>
      </div>

      {/* Results */}
      <div id="search-results" className="flex-1 overflow-y-auto px-6 py-3" ref={resultsRef} role="listbox" aria-label="Search results">
        {query.trim().length === 0 && !loading && (
          <EmptyState
            className="h-full"
            icon={
              <svg className="w-12 h-12" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1}>
                <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
              </svg>
            }
            title="Type to search your vault"
            subtitle="Press / to focus search"
          />
        )}

        {query.trim().length > 0 && results.length === 0 && !loading && !error && (
          <EmptyState
            className="h-full"
            title={`No results for "${debouncedQuery}"`}
          />
        )}

        {error && (
          <div className="flex flex-col items-center justify-center h-full text-vault-error">
            <svg className="w-8 h-8 mb-2" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
            </svg>
            <p className="text-sm">{error}</p>
          </div>
        )}

        {results.map((result, idx) => (
          <button
            key={result.id}
            id={`result-${idx}`}
            role="option"
            aria-selected={selectedIndex === idx}
            onClick={() => navigate(`/note/${encodeURIComponent(result.id)}`)}
            onMouseEnter={() => setSelectedIndex(idx)}
            className={`w-full text-left p-3 rounded-lg mb-1 transition-colors group ${
              selectedIndex === idx
                ? 'bg-vault-bg-tertiary ring-1 ring-vault-accent/30'
                : 'hover:bg-vault-bg-hover'
            }`}
          >
            <div className="flex items-center gap-2 mb-1">
              <h3 className="text-sm font-medium text-vault-text-primary group-hover:text-vault-accent transition-colors truncate">
                {result.title}
              </h3>
              <span className={`type-badge ${typeBadgeClass(result.type)}`}>{result.type}</span>
              {result.project && (
                <span className="text-xs text-vault-text-muted">{result.project}</span>
              )}
            </div>
            <p className="text-xs text-vault-text-secondary line-clamp-2 leading-relaxed">
              {result.snippet}
            </p>
            <p className="text-xs text-vault-text-muted mt-1.5 font-mono truncate">
              {result.path}
            </p>
          </button>
        ))}
      </div>

      {/* Status bar */}
      {results.length > 0 && (
        <div className="border-t border-vault-border px-6 py-2 text-xs text-vault-text-muted flex justify-between">
          <span>{results.length} result{results.length !== 1 ? 's' : ''}</span>
          <span>Use <kbd className="px-1 py-0.5 bg-vault-bg-tertiary rounded text-[10px]">↑</kbd> <kbd className="px-1 py-0.5 bg-vault-bg-tertiary rounded text-[10px]">↓</kbd> to navigate, <kbd className="px-1 py-0.5 bg-vault-bg-tertiary rounded text-[10px]">Enter</kbd> to open</span>
        </div>
      )}
    </div>
  );
};

export default SearchView;
