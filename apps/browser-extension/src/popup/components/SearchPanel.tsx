import { useState, useCallback } from 'react';
import { searchVault } from '@shared/api';
import type { SearchResult } from '@shared/types';

export function SearchPanel() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [searched, setSearched] = useState(false);
  const [vectorEnabled, setVectorEnabled] = useState(false);
  const [hybridWeight, setHybridWeight] = useState(0.5);

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;
    setLoading(true); setError(''); setSearched(true);
    try {
      const params = {
        q: query,
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
  }, [query, vectorEnabled, hybridWeight]);

  return (
    <div style={{ padding: '14px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
      <div style={{ display: 'flex', gap: '8px' }}>
        <input type="text" value={query} onChange={(e) => setQuery(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && handleSearch()}
          placeholder="Search your vault..."
          style={{ flex: 1, padding: '8px 10px', background: '#1a1d27', color: '#e4e6eb', border: '1px solid #2a2d3a', borderRadius: '6px', outline: 'none', fontSize: '13px' }} />
        <button onClick={handleSearch} disabled={loading || !query.trim()}
          style={{ padding: '8px 14px', background: '#4f7cff', color: '#fff', border: 'none', borderRadius: '6px', fontSize: '13px', fontWeight: 600 }}>
          {loading ? '...' : 'Search'}
        </button>
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
      {error && (
        <div style={{ padding: '8px 12px', background: 'rgba(239,68,68,0.1)', border: '1px solid rgba(239,68,68,0.3)', borderRadius: '6px', color: '#ef4444', fontSize: '12px' }}>{error}</div>
      )}
      {searched && !loading && results.length === 0 && !error && (
        <div style={{ textAlign: 'center', color: '#6b7280', fontSize: '13px', padding: '20px' }}>No results found.</div>
      )}
      {results.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {results.map((r) => (
            <div key={r.id} style={{ padding: '10px 12px', background: '#1a1d27', borderRadius: '6px', border: '1px solid #2a2d3a' }}>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '4px' }}>
                <span style={{ fontSize: '13px', fontWeight: 600, color: '#e4e6eb' }}>{r.title}</span>
                <span style={{ fontSize: '10px', padding: '2px 6px', background: 'rgba(79,124,255,0.15)', color: '#4f7cff', borderRadius: '4px', textTransform: 'uppercase' }}>{r.type}</span>
              </div>
              {r.snippet && <p style={{ fontSize: '12px', color: '#6b7280', lineHeight: '1.4', overflow: 'hidden', display: '-webkit-box', WebkitLineClamp: 2, WebkitBoxOrient: 'vertical' }}>{r.snippet}</p>}
              <div style={{ fontSize: '11px', color: '#4f7cff', marginTop: '4px' }}>{r.path}</div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
