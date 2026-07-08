import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import NoteEditor from '../NoteEditor';

const mockCreateNote = vi.hoisted(() => vi.fn());

vi.mock('@/api/client', () => ({
  api: {
    createNote: mockCreateNote,
  },
  ApiError: class extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.status = status;
    }
  },
}));

describe('NoteEditor', () => {
  beforeEach(() => {
    mockCreateNote.mockReset();
  });

  it('does not submit when the title is empty', async () => {
    const user = userEvent.setup();
    const onCreated = vi.fn();

    render(<NoteEditor onCreated={onCreated} />);

    const submitButton = screen.getByRole('button', { name: /create note/i });
    await user.click(submitButton);

    expect(mockCreateNote).not.toHaveBeenCalled();
    expect(onCreated).not.toHaveBeenCalled();
  });

  it('creates a note with trimmed fields', async () => {
    const user = userEvent.setup();
    const onCreated = vi.fn();
    mockCreateNote.mockResolvedValueOnce({ id: 'note-1', path: 'notes/note-1.md' });

    render(<NoteEditor onCreated={onCreated} />);

    await user.type(screen.getByPlaceholderText('Note title'), '  My Note  ');
    await user.type(screen.getByPlaceholderText('Project name (optional)'), '  work  ');
    await user.type(screen.getByPlaceholderText('tag1, tag2, tag3'), '  a,  b,  ');

    await user.click(screen.getByRole('button', { name: /meeting/i }));
    await user.click(screen.getByRole('button', { name: /create note/i }));

    expect(mockCreateNote).toHaveBeenCalledWith({
      type: 'meeting',
      title: 'My Note',
      project: 'work',
      tags: ['a', 'b'],
    });
    expect(onCreated).toHaveBeenCalledWith('note-1', 'notes/note-1.md');
  });

  it('surfaces create errors', async () => {
    const user = userEvent.setup();
    mockCreateNote.mockRejectedValueOnce(new Error('Server error'));

    render(<NoteEditor onCreated={vi.fn()} />);

    await user.type(screen.getByPlaceholderText('Note title'), 'My Note');
    await user.click(screen.getByRole('button', { name: /create note/i }));

    expect(await screen.findByText('Server error')).toBeInTheDocument();
  });

  it('calls onCancel when the cancel button is clicked', async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();

    render(<NoteEditor onCancel={onCancel} />);
    await user.click(screen.getByRole('button', { name: /cancel/i }));

    expect(onCancel).toHaveBeenCalled();
  });
});
