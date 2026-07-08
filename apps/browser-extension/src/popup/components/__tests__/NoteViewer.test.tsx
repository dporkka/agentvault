// @vitest-environment jsdom
import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, cleanup } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';

const mockGetNote = vi.hoisted(() => vi.fn());
vi.mock('@shared/api', () => ({
  getNote: mockGetNote,
}));

import { NoteViewer } from '../NoteViewer';

describe('NoteViewer', () => {
  beforeEach(() => {
    cleanup();
    mockGetNote.mockReset();
  });

  it('shows a loading state while fetching', () => {
    mockGetNote.mockImplementation(() => new Promise(() => { /* never resolves */ }));
    render(<NoteViewer id="n1" onBack={vi.fn()} />);
    expect(screen.getByText('Loading note...')).toBeInTheDocument();
  });

  it('renders the note when loaded', async () => {
    mockGetNote.mockResolvedValue({
      id: 'n1',
      title: 'Test Note',
      type: 'note',
      path: 'notes/n1.md',
      content: 'note body',
      tags: ['a', 'b'],
      project: 'work',
      status: 'active',
    });
    render(<NoteViewer id="n1" onBack={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Test Note')).toBeInTheDocument());
    expect(screen.getByText('note body')).toBeInTheDocument();
    expect(screen.getByText('work')).toBeInTheDocument();
    expect(screen.getByText('a')).toBeInTheDocument();
    expect(screen.getByText('b')).toBeInTheDocument();
  });

  it('shows a not-found message when the note is missing', async () => {
    mockGetNote.mockResolvedValue(null);
    render(<NoteViewer id="n1" onBack={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('Note not found.')).toBeInTheDocument());
  });

  it('displays an error when fetching fails', async () => {
    mockGetNote.mockRejectedValue(new Error('server down'));
    render(<NoteViewer id="n1" onBack={vi.fn()} />);
    await waitFor(() => expect(screen.getByText('server down')).toBeInTheDocument());
  });

  it('calls onBack when the back button is clicked', async () => {
    const onBack = vi.fn();
    mockGetNote.mockResolvedValue({
      id: 'n1',
      title: 'Test Note',
      type: 'note',
      path: 'notes/n1.md',
      content: 'note body',
      tags: [],
    });
    render(<NoteViewer id="n1" onBack={onBack} />);
    await waitFor(() => expect(screen.getByText('Test Note')).toBeInTheDocument());
    screen.getByRole('button', { name: 'Back to list' }).click();
    expect(onBack).toHaveBeenCalled();
  });
});
