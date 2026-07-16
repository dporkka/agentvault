import React, { useState, useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/client';
import EmptyState from '@/components/EmptyState';
import type { SearchResult } from '@agentvault/contract';

interface TagFrequency {
  tag: string;
  count: number;
}

const TagBrowser: React.FC = () => {
  const navigate = useNavigate();
  const [tags, setTags] = useState<TagFrequency[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadTags() {
      try {
        // Fetch recent notes and search results to collect tags
        const [recent, searchResults] = await Promise.all([
          api.getRecent({ limit: 200 }),
          api.search({ q: '' }).catch(() => [] as SearchResult[]),
        ]);

        if (cancelled) return;

        const freqMap = new Map<string, number>();
        const addTags = (notes: SearchResult[]) => {
          for (const note of notes) {
            if (note.tags) {
              for (const tag of note.tags) {
                freqMap.set(tag, (freqMap.get(tag) || 0) + 1);
              }
            }
          }
        };

        addTags(recent);
        addTags(searchResults);

        const sorted = Array.from(freqMap.entries())
          .map(([tag, count]) => ({ tag, count }))
          .sort((a, b) => b.count - a.count);

        setTags(sorted);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Failed to load tags');
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadTags();
    return () => { cancelled = true; };
  }, []);

  const maxCount = tags.length > 0 ? tags[0].count : 1;

  const getTagSize = (count: number): string => {
    const ratio = count / maxCount;
    if (ratio >= 0.8) return 'text-lg font-bold';
    if (ratio >= 0.5) return 'text-base font-semibold';
    if (ratio >= 0.2) return 'text-sm font-medium';
    return 'text-xs font-normal';
  };

  const getTagOpacity = (count: number): string => {
    const ratio = count / maxCount;
    return ratio >= 0.5 ? 'opacity-100' : ratio >= 0.2 ? 'opacity-80' : 'opacity-60';
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-sm text-vault-text-muted">Loading tags...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3">
        <p className="text-sm text-vault-error">{error}</p>
      </div>
    );
  }

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b border-vault-border px-6 py-4">
        <h1 className="text-lg font-semibold text-vault-text-primary">Tags</h1>
        <p className="text-sm text-vault-text-muted mt-1">
          {tags.length > 0
            ? `${tags.length} tag${tags.length !== 1 ? 's' : ''} across your vault`
            : 'Browse notes by tag'}
        </p>
      </div>

      {/* Tag cloud */}
      <div className="flex-1 overflow-y-auto px-6 py-6">
        {tags.length === 0 ? (
          <EmptyState
            className="h-full"
            title="No tags yet"
            subtitle="Tags are extracted from your notes. Create a note with tags to see them here."
          />
        ) : (
          <div className="flex flex-wrap gap-3">
            {tags.map(({ tag, count }) => (
              <button
                key={tag}
                onClick={() => navigate(`/search?tag=${encodeURIComponent(tag)}`)}
                className={`px-3 py-1.5 rounded-full border border-vault-border bg-vault-bg-secondary hover:bg-vault-bg-hover hover:border-vault-accent transition-colors ${getTagSize(tag.length > 8 ? Math.min(count, maxCount * 0.5) : count)} text-vault-text-primary ${getTagOpacity(count)}`}
                title={`${count} note${count !== 1 ? 's' : ''}`}
              >
                #{tag}
                <span className="ml-1.5 text-vault-text-muted text-xs">({count})</span>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
};

export default TagBrowser;
