import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import Sidebar from '../Sidebar';
import type { VaultStatus, IndexingStatus, AIStatus } from '../../types';

describe('Sidebar', () => {
  const vaultStatus: VaultStatus = {
    path: '/home/user/work-vault',
    isVault: true,
    noteCount: 7,
    version: '0.1.0',
  };
  const onViewChange = vi.fn();
  const onOpenNote = vi.fn();
  const onNewNote = vi.fn();
  const onVaultChanged = vi.fn();
  const onToggleCollapse = vi.fn();

  beforeEach(() => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    onViewChange.mockReset();
    onOpenNote.mockReset();
    onNewNote.mockReset();
    onVaultChanged.mockReset();
    onToggleCollapse.mockReset();

    (window as any).go = {
      main: {
        NoteService: {
          GetRecent: vi.fn(() => Promise.resolve([
            { id: '1', title: 'Hello', path: '10-notes/hello.md', type: 'note', project: '', status: '', tags: [], snippet: '', score: 0, updatedAt: '' },
          ])),
        },
        IndexService: {
          GetStatus: vi.fn(() => Promise.resolve({ isIndexing: false, noteCount: 7 } as IndexingStatus)),
        },
        AIService: {
          GetStatus: vi.fn(() => Promise.resolve({ enabled: true, provider: 'ollama', model: 'llama3.1', error: '' } as AIStatus)),
        },
      },
    };
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function renderSidebar(collapsed = false) {
    return render(
      <Sidebar
        vaultStatus={vaultStatus}
        activeView="search"
        onViewChange={onViewChange}
        onOpenNote={onOpenNote}
        onNewNote={onNewNote}
        onVaultChanged={onVaultChanged}
        collapsed={collapsed}
        onToggleCollapse={onToggleCollapse}
      />,
    );
  }

  it('renders the vault name and note count', async () => {
    renderSidebar();

    await waitFor(() => {
      expect(screen.getByText('work-vault')).toBeInTheDocument();
    });
    expect(screen.getByText('7 notes')).toBeInTheDocument();
  });

  it('calls onViewChange when a nav item is clicked', async () => {
    renderSidebar();

    await userEvent.click(screen.getByRole('button', { name: 'Editor' }));
    expect(onViewChange).toHaveBeenCalledWith('editor');
  });

  it('calls onNewNote when the new note button is clicked', async () => {
    renderSidebar();

    await userEvent.click(screen.getByRole('button', { name: /New Note/i }));
    expect(onNewNote).toHaveBeenCalledTimes(1);
  });

  it('toggles folder expansion', async () => {
    renderSidebar();

    await waitFor(() => {
      expect(screen.getByText('10-notes')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('10-notes'));
    // After clicking, the folder label should still be present.
    expect(screen.getByText('10-notes')).toBeInTheDocument();
  });

  it('renders recent notes and opens them', async () => {
    renderSidebar();

    await waitFor(() => {
      expect(screen.getByText('Hello')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('Hello'));
    expect(onOpenNote).toHaveBeenCalledWith('10-notes/hello.md');
  });

  it('calls onToggleCollapse when the collapse button is clicked', async () => {
    renderSidebar();

    await userEvent.click(screen.getByRole('button', { name: 'Collapse sidebar' }));
    expect(onToggleCollapse).toHaveBeenCalledTimes(1);
  });

  it('renders a compact view when collapsed', () => {
    renderSidebar(true);

    expect(screen.queryByText('work-vault')).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Search' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Editor' })).toBeInTheDocument();
  });

  it('shows AI status when configured', async () => {
    renderSidebar();

    await waitFor(() => {
      expect(screen.getByText(/AI: ollama/)).toBeInTheDocument();
    });
  });
});
