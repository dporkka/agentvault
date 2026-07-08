import { useState, useCallback } from 'react';
import { searchVault, getNote } from '@shared/api';
import type { SearchResult, NoteDetail } from '@shared/types';

const NOTE_TYPES = [
  { value: '', label: 'All types' },
  { value: 'note', label: 'Note' },
  { value: 'webpage', label: 'Webpage' },
  { value: 'selection', label: 'Selection' },
  { value: 'project', label: 'Project' },
  { value: 'decision', label: 'Decision' },
  { value: 'person', label: 'Person' },
  { value: 'company', label: 'Company' },
  { value: 'research', label: 'Research' },
  { value: 'prompt', label: 'Prompt' },
];

const STATUSES = [
  { value: '', label: 'Any status' },
  { value: 'active', label: 'Active' },
  { value: 'archived', label: 'Archived' },
  { value: 'draft', label: 'Draft' },
  { value: 'stale', label: 'Stale' },
];

export function SearchPanel() {
  const [query, setQuery] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [searched, setSearched] = useState(false);
  const [vectorEnabled, setVectorEnabled] = useState(false);
  const [hybridWeight, setHybridWeight] = useState(0.5);
  const [selectedNote, setSelectedNote] = useState<NoteDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState('');

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    setLoading(true); setError(''); setSearched(true);
    setSelectedNote(null); setDetailError('');
    try {
      const params = {
        q: query,
        type: typeFilter || undefined,
        status: statusFilter || undefined,
        vector: vectorEnabled || undefined,
        hybridWeight: vectorEnabled ? hybridWeight : undefined,
      };
      setResults(await searchVault(params));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Search failed');
      setResults([]);
    } finally {
      setLoading(false);
    }
  }, [query, typeFilter, statusFilter, vectorEnabled, hybridWeight]);

  const openDetail = useCallback(async (id: string) => {
    setDetailLoading(true); setDetailError('');
    try {
      const note = await getNote(id);
      if (!note) {
        setDetailError('Note not found or server unavailable');
        setSelectedNote(null);
      } else {
        setSelectedNote(note);
      }
    } catch (err) {
      setDetailError(err instanceof Error ? err.message : 'Failed to load note');
      setSelectedNote(null);
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const closeDetail = useCallback(() => {
    setSelectedNote(null);
    setDetailError('');
  }, []);

  return (
    <div className="search-panel">
      <div className="search-panel__row">
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          placeholder="Search your vault..."
          className="input flex-1"
        />
        <button
          onClick={handleSearch}
          disabled={loading || !query.trim()}
          className="btn btn-primary"
        >
          {loading ? '...' : 'Search'}
        </button>
      </div>

      <div className="search-panel__filters">
        <div className="search-panel__row">
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            className="select flex-1"
            aria-label="Filter by type"
          >
            {NOTE_TYPES.map((t) => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </select>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="select flex-1"
            aria-label="Filter by status"
          >
            {STATUSES.map((s) => (
              <option key={s.value} value={s.value}>{s.label}</option>
            ))}
          </select>
        </div>
        <div className="search-panel__toggles">
          <label className="search-panel__checkbox">
            <input
              type="checkbox"
              checked={vectorEnabled}
              onChange={(e) => setVectorEnabled(e.target.checked)}
            />
            Vector search
          </label>
          {vectorEnabled && (
            <div className="search-panel__row">
              <span>Hybrid weight</span>
              <input
                type="range"
                min={0}
                max={1}
                step={0.1}
                value={hybridWeight}
                onChange={(e) => setHybridWeight(parseFloat(e.target.value))}
                className="range-sm"
              />
              <span className="mono-value">{hybridWeight.toFixed(1)}</span>
            </div>
          )}
        </div>
      </div>

      {error && (
        <div className="banner banner-error">{error}</div>
      )}
      {detailError && (
        <div className="banner banner-error">{detailError}</div>
      )}

      {detailLoading && (
        <div className="empty-state">Loading note...</div>
      )}

      {searched && !loading && results.length === 0 && !error && !selectedNote && (
        <div className="empty-state">No results found.</div>
      )}

      {results.length > 0 && !selectedNote && (
        <div className="result-list">
          {results.map((r) => (
            <button
              key={r.id}
              onClick={() => openDetail(r.id)}
              className="result-card"
            >
              <div className="result-card__header">
                <span className="result-card__title">{r.title}</span>
                <span className="badge">{r.type}</span>
              </div>
              {r.snippet && <p className="result-card__snippet">{r.snippet}</p>}
              <div className="result-card__meta">
                <span className="result-card__path">{r.path}</span>
                {r.project && <span className="tag">{r.project}</span>}
                {r.status && r.status !== 'active' && <span className="tag">{r.status}</span>}
                {r.tags?.map((tag) => <span key={tag} className="tag">{tag}</span>)}
              </div>
            </button>
          ))}
        </div>
      )}

      {selectedNote && (
        <div className="modal-overlay">
          <div
            className="modal"
            role="dialog"
            aria-modal="true"
          >
            <div className="modal__header">
              <div className="modal__header-left">
                <span className="modal__title">{selectedNote.title}</span>
                <span className="badge">{selectedNote.type}</span>
              </div>
              <button
                onClick={closeDetail}
                aria-label="Close note detail"
                className="icon-btn icon-btn--lg"
              >
                ×
              </button>
            </div>
            <div className="modal__body">
              <div className="meta-tags">
                <span className="result-card__path">{selectedNote.path}</span>
                {selectedNote.project && <span className="tag">{selectedNote.project}</span>}
                {selectedNote.status && <span className="tag">{selectedNote.status}</span>}
                {selectedNote.tags?.map((tag) => <span key={tag} className="tag">{tag}</span>)}
              </div>
              <pre className="modal__content">
                {selectedNote.content}
              </pre>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
