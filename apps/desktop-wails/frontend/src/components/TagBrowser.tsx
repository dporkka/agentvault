import { useState, useEffect } from 'react';
import type { SearchResult } from '../types';

interface Props {
  onOpenNote: (path: string) => void;
  onSearchTag: (tag: string) => void;
}

interface TagFrequency {
  tag: string;
  count: number;
}

export default function TagBrowser({ onOpenNote, onSearchTag }: Props) {
  const [tags, setTags] = useState<TagFrequency[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selectedTag, setSelectedTag] = useState<string | null>(null);
  const [tagNotes, setTagNotes] = useState<SearchResult[]>([]);
  const [notesLoading, setNotesLoading] = useState(false);

  useEffect(() => {
    let cancelled = false;

    async function loadTags() {
      try {
        const [recent, searchResults] = await Promise.all([
          window.go.main.NoteService.GetRecent(200),
          window.go.main.NoteService.Search('', '', '', false, 0, 200),
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

  useEffect(() => {
    if (!selectedTag) {
      setTagNotes([]);
      return;
    }

    let cancelled = false;
    setNotesLoading(true);

    async function loadTagNotes() {
      try {
        // Search for notes that have this tag in their content
        const results = await window.go.main.NoteService.Search(selectedTag!, '', '', false, 0, 100);
        if (cancelled) return;
        // Filter results that actually have this tag (Wails search is full-text, not tag-filtered)
        const filtered = results.filter((r) => r.tags && r.tags.includes(selectedTag!));
        setTagNotes(filtered);
      } catch {
        if (!cancelled) setTagNotes([]);
      } finally {
        if (!cancelled) setNotesLoading(false);
      }
    }

    loadTagNotes();
    return () => { cancelled = true; };
  }, [selectedTag]);

  const maxCount = tags.length > 0 ? tags[0].count : 1;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-sm text-text-muted">Loading tags...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3">
        <p className="text-sm text-red-400">{error}</p>
      </div>
    );
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {/* Header */}
      <div className="border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold text-text-primary">Tags</h1>
        <p className="text-sm text-text-muted mt-1">
          {tags.length > 0
            ? `${tags.length} tag${tags.length !== 1 ? 's' : ''} across your vault`
            : 'Browse notes by tag'}
        </p>
      </div>

      <div className="flex-1 overflow-y-auto px-6 py-6">
        {tags.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <p className="text-sm text-text-muted">No tags yet. Create a note with tags to see them here.</p>
          </div>
        ) : (
          <>
            {/* Tag cloud */}
            <div className="flex flex-wrap gap-3">
              {tags.map(({ tag, count }) => {
                const ratio = count / maxCount;
                const sizeClass =
                  ratio >= 0.8 ? 'text-lg font-bold' :
                  ratio >= 0.5 ? 'text-base font-semibold' :
                  ratio >= 0.2 ? 'text-sm font-medium' : 'text-xs font-normal';
                const opacityClass = ratio >= 0.5 ? 'opacity-100' : ratio >= 0.2 ? 'opacity-80' : 'opacity-60';

                return (
                  <button
                    key={tag}
                    onClick={() => setSelectedTag(selectedTag === tag ? null : tag)}
                    className={`px-3 py-1.5 rounded-full border border-border bg-bg-secondary hover:bg-bg-hover hover:border-accent transition-colors ${sizeClass} text-text-primary ${opacityClass} ${
                      selectedTag === tag ? 'border-accent bg-accent/10 ring-1 ring-accent/30' : ''
                    }`}
                    title={`${count} note${count !== 1 ? 's' : ''}`}
                  >
                    #{tag}
                    <span className="ml-1.5 text-text-muted text-xs">({count})</span>
                  </button>
                );
              })}
            </div>

            {/* Selected tag notes */}
            {selectedTag && (
              <div className="mt-6 animate-fade-in">
                <div className="flex items-center justify-between mb-3">
                  <h2 className="text-sm font-semibold text-text-primary">
                    Notes tagged <span className="text-accent">#{selectedTag}</span>
                  </h2>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => onSearchTag(selectedTag)}
                      className="flex items-center gap-1.5 text-xs text-accent hover:text-accent-hover transition-colors"
                    >
                      <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
                        <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
                      </svg>
                      Search all
                    </button>
                    <button
                      onClick={() => setSelectedTag(null)}
                      className="text-xs text-text-muted hover:text-text-primary transition-colors"
                    >
                      Close
                    </button>
                  </div>
                </div>

                {notesLoading ? (
                  <div className="flex items-center justify-center py-6">
                    <div className="text-sm text-text-muted">Loading...</div>
                  </div>
                ) : tagNotes.length === 0 ? (
                  <p className="text-sm text-text-muted">No notes found with this tag.</p>
                ) : (
                  <div className="space-y-1">
                    {tagNotes.map((note) => (
                      <button
                        key={note.id}
                        onClick={() => onOpenNote(note.path)}
                        className="w-full text-left flex items-center gap-3 px-3 py-2 rounded-md hover:bg-bg-hover transition-colors group"
                      >
                        <span className="px-2 py-0.5 text-[10px] font-medium rounded-full border capitalize bg-blue-500/10 text-blue-400 border-blue-500/20">
                          {note.type}
                        </span>
                        <span className="text-sm text-text-primary group-hover:text-accent transition-colors truncate flex-1">
                          {note.title}
                        </span>
                      </button>
                    ))}
                  </div>
                )}
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}
