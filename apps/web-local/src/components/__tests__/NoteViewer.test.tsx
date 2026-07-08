import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import NoteViewer from '../NoteViewer';
import type { NoteDetail } from '@agentvault/contract';

const note: NoteDetail = {
  id: 'note-1',
  title: 'My Note',
  path: 'notes/note-1.md',
  type: 'note',
  project: 'work',
  status: 'active',
  tags: ['a'],
  content: '# Heading\n\nSome body',
};

describe('NoteViewer', () => {
  it('renders note title, type badge, and path', () => {
    render(
      <MemoryRouter>
        <NoteViewer note={note} />
      </MemoryRouter>,
    );

    expect(screen.getByText('My Note')).toBeInTheDocument();
    expect(screen.getByText('note')).toBeInTheDocument();
    expect(screen.getByText('notes/note-1.md')).toBeInTheDocument();
  });

  it('toggles between rendered markdown and raw content', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <NoteViewer note={note} />
      </MemoryRouter>,
    );

    expect(screen.getByRole('button', { name: /raw/i })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: /raw/i }));

    expect(screen.getByRole('button', { name: /rendered/i })).toBeInTheDocument();
    const pre = screen.getByText((content) => content.includes('# Heading') && content.includes('Some body'));
    expect(pre.tagName).toBe('PRE');
  });

  it('uses default badge class for unknown note types', () => {
    render(
      <MemoryRouter>
        <NoteViewer note={{ ...note, type: 'unknown' }} />
      </MemoryRouter>,
    );
    expect(screen.getByText('unknown')).toBeInTheDocument();
  });
});
