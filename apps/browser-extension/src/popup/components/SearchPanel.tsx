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

  const inputStyle: React.CSSProperties = {
    padding: '8px 10px',
    background: '#1a1d27',
    color: '#e4e6eb',
    border: '1px solid #2a2d3a',
    borderRadius: '6px',
    outline: 'none',
    fontSize: '13px',
    width: '100%',
    boxSizing: 'border-box',
  };

  const selectStyle: React.CSSProperties = {
    ...inputStyle,
    cursor: 'pointer',
    appearance: 'auto',
  };

  const tagStyle: React.CSSProperties = {
    fontSize: '10px',
    padding: '2px 6px',
    background: 'rgba(79,124,255,0.12)',
    color: '#4f7cff',
    borderRadius: '4px',
  };

  return (
    <div style={{ padding: '14px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <div style={{ display: 'flex', gap: '8px' }}>
        <input
          type="text"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          placeholder="Search your vault..."
          style={{ flex: 1, ...inputStyle }}
        />
        <button
          onClick={handleSearch}
          disabled={loading || !query.trim()}
          style={{ padding: '8px 14px', background: '#4f7cff', color: '#fff', border: 'none', borderRadius: '6px', fontSize: '13px', fontWeight: 600, cursor: loading || !query.trim() ? 'not-allowed' : 'pointer', opacity: loading || !query.trim() ? 0.6 : 1 }}
        >
          {loading ? '...' : 'Search'}
        </button>
      </div>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '10px' }}>
        <div style={{ display: 'flex', gap: '8px' }}>
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value)}
            style={{ flex: 1, ...selectStyle }}
            aria-label="Filter by type"
          >
            {NOTE_TYPES.map((t) => (
              <option key={t.value} value={t.value}>{t.label}</option>
            ))}
          </select>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            style={{ flex: 1, ...selectStyle }}
            aria-label="Filter by status"
          >
            {STATUSES.map((s) => (
              <option key={s.value} value={s.value}>{s.label}</option>
            ))}
          </select>
        </div>
        <div style={{ display: 'flex', alignItems: 'center', gap: '14px', fontSize: '12px', color: '#9ca3af' }}>
          <label style={{ display: 'flex', alignItems: 'center', gap: '6px', cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={vectorEnabled}
              onChange={(e) => setVectorEnabled(e.target.checked)}
            />
            Vector search
          </label>
          {vectorEnabled && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
              <span>Hybrid weight</span>
              <input
                type="range"
                min={0}
                max={1}
                step={0.1}
                value={hybridWeight}
                onChange={(e) => setHybridWeight(parseFloat(e.target.value))}
                style={{ width: '96px' }}
              />
              <span style={{ fontFamily: 'monospace', minWidth: '28px' }}>{hybridWeight.toFixed(1)}</span>
            </div>
          )}
        </div>
      </div>

      {error && (
        <div style={{ padding: '8px 12px', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: '6px', color: '#ef4444', fontSize: '12px' }}>{error}</div>
      )}
      {detailError && (
        <div style={{ padding: '8px 12px', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: '6px', color: '#ef4444', fontSize: '12px' }}>{detailError}</div>
      )}

      {detailLoading && (
        <div style={{ textAlign: 'center', color: '#6b7280', fontSize: '13px', padding: '20px' }}>Loading note...</div>
      )}

      {searched && !loading && results.length === 0 && !error && !selectedNote && (
        <div style={{ textAlign: 'center', color: '#6b7280', fontSize: '13px', padding: '20px' }}>No results found.</div>
      )}

      {results.length > 0 && !selectedNote && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {results.map((r) => (
            <button
              key={r.id}
              onClick={() => openDetail(r.id)}
              style={{ textAlign: 'left', padding: '10px 12px', background: '#1a1d27', borderRadius: '6px', border: '1px solid #2a2d3a', cursor: 'pointer' }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '4px', gap: '8px' }}>
                <span style={{ fontSize: '13px', fontWeight: 600, color: '#e4e6eb', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{r.title}</span>
                <span style={{ fontSize: '10px', padding: '2px 6px', background: 'rgba(79,124,255,0.15)', color: '#4f7cff', borderRadius: '4px', textTransform: 'uppercase', flexShrink: 0 }}>{r.type}</span>
              </div>
              {r.snippet && <p style={{ fontSize: '12px', color: '#6b7280', lineHeight: '1.4', overflow: 'hidden', display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical' }}>{r.snippet}</p>}
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginTop: '6px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', color: '#4f7cff' }}>{r.path}</span>
                {r.project && <span style={tagStyle}>{r.project}</span>}
                {r.status && r.status !== 'active' && <span style={tagStyle}>{r.status}</span>}
                {r.tags?.map((tag) => <span key={tag} style={tagStyle}>{tag}</span>)}
              </div>
            </button>
          ))}
        </div>
      )}

      {selectedNote && (
        <div style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.6)', zIndex: 100, display: 'flex', alignItems: 'center', justifyContent: 'center', padding: '12px' }}>
          <div
            style={{
              width: '100%',
              maxWidth: '356px',
              maxHeight: 'calc(100vh - 24px)',
              background: '#0f1117',
              border: '1px solid #2a2d3a',
              borderRadius: '8px',
              display: 'flex',
              flexDirection: 'column',
              overflow: 'hidden',
            }}
            role="dialog"
            aria-modal="true"
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '12px 14px', borderBottom: '1px solid #2a2d3a', gap: '12px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0 }}>
                <span style={{ fontSize: '14px', fontWeight: 600, color: '#e4e6eb', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{selectedNote.title}</span>
                <span style={{ fontSize: '10px', padding: '2px 6px', background: 'rgba(79,124,255,0.15)', color: '#4f7cff', borderRadius: '4px', textTransform: 'uppercase', flexShrink: 0 }}>{selectedNote.type}</span>
              </div>
              <button
                onClick={closeDetail}
                aria-label="Close note detail"
                style={{ background: 'transparent', border: 'none', color: '#6b7280', fontSize: '16px', cursor: 'pointer', padding: '4px', lineHeight: 1 }}
              >
                ×
              </button>
            </div>
            <div style={{ padding: '12px 14px', overflowY: 'auto', display: 'flex', flexDirection: 'column', gap: '10px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                <span style={{ fontSize: '11px', color: '#4f7cff' }}>{selectedNote.path}</span>
                {selectedNote.project && <span style={tagStyle}>{selectedNote.project}</span>}
                {selectedNote.status && <span style={tagStyle}>{selectedNote.status}</span>}
                {selectedNote.tags?.map((tag) => <span key={tag} style={tagStyle}>{tag}</span>)}
              </div>
              <pre style={{ margin: 0, fontSize: '12px', lineHeight: '1.5', color: '#e4e6eb', whiteSpace: 'pre-wrap', wordBreak: 'break-word', fontFamily: 'inherit' }}>
                {selectedNote.content}
              </pre>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
