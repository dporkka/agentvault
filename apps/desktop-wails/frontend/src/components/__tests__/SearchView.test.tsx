import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import SearchView from '../SearchView';
import type { SearchResult } from '../../types';

describe('SearchView', () => {
  const onOpenNote = vi.fn();
  let recentResults: SearchResult[] = [];
  let searchResults: SearchResult[] = [];

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    onOpenNote.mockReset();
    recentResults = [
      { id: '1', title: 'Recent Note', path: '10-notes/recent.md', type: 'note', project: '', status: '', tags: [], snippet: 'snippet', score: 0, updatedAt: '' },
    ];
    searchResults = [
      { id: '2', title: 'Search Result', path: '10-notes/result.md', type: 'decision', project: 'work', status: '', tags: [], snippet: 'matched', score: 0.95, updatedAt: '' },
    ];

    (window as any).go = {
      main: {
        NoteService: {
          GetRecent: vi.fn(() => Promise.resolve(recentResults)),
          Search: vi.fn(() => Promise.resolve(searchResults)),
        },
      },
    };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('loads recent notes on mount', async () => {
    render(<SearchView onOpenNote={onOpenNote} />);

    await waitFor(() => {
      expect(screen.getByText('Recent Note')).toBeInTheDocument();
    });
  });

  it('debounces search input and calls the search service', async () => {
    render(<SearchView onOpenNote={onOpenNote} />);

    const input = screen.getByPlaceholderText(/Search notes/);
    await userEvent.type(input, 'hello');

    vi.advanceTimersByTime(250);

    await waitFor(() => {
      expect(window.go.main.NoteService.Search).toHaveBeenCalledWith(
        'hello',
        '',
        '',
        false,
        0.5,
        30,
      );
    });
  });

  it('does not search for an empty query', async () => {
    render(<SearchView onOpenNote={onOpenNote} />);

    const input = screen.getByPlaceholderText(/Search notes/);
    await userEvent.clear(input);

    vi.advanceTimersByTime(250);

    expect(window.go.main.NoteService.Search).not.toHaveBeenCalled();
  });

  it('opens a result when clicked', async () => {
    render(<SearchView onOpenNote={onOpenNote} />);

    await waitFor(() => {
      expect(screen.getByText('Recent Note')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('Recent Note'));
    expect(onOpenNote).toHaveBeenCalledWith('10-notes/recent.md');
  });

  it('updates the type filter and passes it to search', async () => {
    render(<SearchView onOpenNote={onOpenNote} />);

    await userEvent.click(screen.getByRole('button', { name: 'decision' }));

    const input = screen.getByPlaceholderText(/Search notes/);
    await userEvent.type(input, 'pricing');

    vi.advanceTimersByTime(250);

    await waitFor(() => {
      expect(window.go.main.NoteService.Search).toHaveBeenCalledWith(
        'pricing',
        'decision',
        '',
        false,
        0.5,
        30,
      );
    });
  });

  it('toggles vector search and shows hybrid controls', async () => {
    render(<SearchView onOpenNote={onOpenNote} />);

    const vectorButton = screen.getByRole('button', { name: 'Vector' });
    await userEvent.click(vectorButton);

    expect(screen.getByText('Hybrid')).toBeInTheDocument();
    expect(screen.getByText('TopK')).toBeInTheDocument();
  });

  it('navigates results with arrow keys and opens with enter', async () => {
    render(<SearchView onOpenNote={onOpenNote} />);

    await waitFor(() => {
      expect(screen.getByText('Recent Note')).toBeInTheDocument();
    });

    await userEvent.keyboard('{ArrowDown}');
    await userEvent.keyboard('{Enter}');

    expect(onOpenNote).toHaveBeenCalledWith('10-notes/recent.md');
  });

  it('clears the query and blurs input on escape', async () => {
    render(<SearchView onOpenNote={onOpenNote} />);

    const input = screen.getByPlaceholderText(/Search notes/) as HTMLInputElement;
    await userEvent.type(input, 'test');

    await userEvent.keyboard('{Escape}');

    expect(input.value).toBe('');
  });

  it('focuses the search input when / is pressed', async () => {
    render(<SearchView onOpenNote={onOpenNote} />);

    const input = screen.getByPlaceholderText(/Search notes/);
    expect(document.activeElement).not.toBe(input);

    await userEvent.keyboard('/');

    expect(document.activeElement).toBe(input);
  });
});
