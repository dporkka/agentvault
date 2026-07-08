import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, fireEvent } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import SearchView from '../SearchView';

const mockSearch = vi.hoisted(() => vi.fn());

vi.mock('@/api/client', () => ({
  api: {
    search: mockSearch,
  },
}));

vi.mock('@/hooks/useDebounce', () => ({
  useDebounce: (value: string) => value,
}));

describe('SearchView', () => {
  beforeEach(() => {
    mockSearch.mockReset();
    mockSearch.mockResolvedValue([]);
  });

  it('shows the empty state before typing', () => {
    render(
      <MemoryRouter>
        <SearchView />
      </MemoryRouter>,
    );

    expect(screen.getByText('Type to search your vault')).toBeInTheDocument();
  });

  it('searches after the query is entered', async () => {
    mockSearch.mockResolvedValue([
      {
        id: '1',
        title: 'Result',
        path: 'notes/result.md',
        type: 'note',
        project: '',
        status: 'active',
        tags: [],
        snippet: 'snippet',
        score: 1,
        updatedAt: '',
      },
    ]);

    render(
      <MemoryRouter>
        <SearchView />
      </MemoryRouter>,
    );

    const input = screen.getByPlaceholderText('Search notes... (press / to focus)');
    fireEvent.change(input, { target: { value: 'hello' } });

    await waitFor(() => expect(mockSearch).toHaveBeenCalledWith(expect.objectContaining({ q: 'hello' })));
    expect(await screen.findByText('Result')).toBeInTheDocument();
  });

  it('filters by type', async () => {
    mockSearch.mockResolvedValue([]);

    render(
      <MemoryRouter>
        <SearchView />
      </MemoryRouter>,
    );

    fireEvent.click(screen.getByRole('button', { name: /decision/i }));

    const input = screen.getByPlaceholderText('Search notes... (press / to focus)');
    fireEvent.change(input, { target: { value: 'hello' } });

    await waitFor(() =>
      expect(mockSearch).toHaveBeenCalledWith(expect.objectContaining({ q: 'hello', type: 'decision' })),
    );
  });

  it('does not call search for whitespace-only queries', async () => {
    render(
      <MemoryRouter>
        <SearchView />
      </MemoryRouter>,
    );

    const input = screen.getByPlaceholderText('Search notes... (press / to focus)');
    fireEvent.change(input, { target: { value: '   ' } });

    // Wait a tick to ensure any queued effects have run.
    await waitFor(() => expect(mockSearch).not.toHaveBeenCalled(), { timeout: 100 });
  });

  it('surfaces search errors', async () => {
    mockSearch.mockRejectedValue(new Error('Search unavailable'));

    render(
      <MemoryRouter>
        <SearchView />
      </MemoryRouter>,
    );

    const input = screen.getByPlaceholderText('Search notes... (press / to focus)');
    fireEvent.change(input, { target: { value: 'hello' } });

    expect(await screen.findByText('Search unavailable')).toBeInTheDocument();
  });
});
