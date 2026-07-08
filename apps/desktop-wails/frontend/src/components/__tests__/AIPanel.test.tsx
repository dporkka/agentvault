import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import AIPanel from '../AIPanel';
import type { Answer } from '../../types';

describe('AIPanel', () => {
  const onClose = vi.fn();
  const onOpenNote = vi.fn();
  let askResponse: Answer = { answer: 'Test answer', sources: [], confidence: 'high' };

  beforeEach(() => {
    onClose.mockReset();
    onOpenNote.mockReset();
    askResponse = { answer: 'Test answer', sources: [], confidence: 'high' };

    (window as any).go = {
      main: {
        AIService: {
          IsAIEnabled: vi.fn(() => Promise.resolve(true)),
          Ask: vi.fn(() => Promise.resolve(askResponse)),
        },
      },
    };
  });

  it('renders the header and close button', () => {
    render(<AIPanel onClose={onClose} onOpenNote={onOpenNote} vaultPath="/vault" />);

    expect(screen.getByText('AI Assistant')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Close' })).toBeInTheDocument();
  });

  it('calls onClose when the close button is clicked', async () => {
    render(<AIPanel onClose={onClose} onOpenNote={onOpenNote} vaultPath="/vault" />);

    await userEvent.click(screen.getByRole('button', { name: 'Close' }));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('shows a chat message after submitting a question', async () => {
    render(<AIPanel onClose={onClose} onOpenNote={onOpenNote} vaultPath="/vault" />);

    const input = screen.getByPlaceholderText(/Ask about your vault/);
    await userEvent.type(input, 'What is the answer?');
    await userEvent.click(screen.getByRole('button', { name: 'Send' }));

    await waitFor(() => {
      expect(screen.getByText('What is the answer?')).toBeInTheDocument();
    });

    await waitFor(() => {
      expect(screen.getByText('Test answer')).toBeInTheDocument();
    });
  });

  it('shows an error message when Ask fails', async () => {
    (window.go.main.AIService.Ask as any).mockRejectedValue(new Error('Ollama unreachable'));

    render(<AIPanel onClose={onClose} onOpenNote={onOpenNote} vaultPath="/vault" />);

    const input = screen.getByPlaceholderText(/Ask about your vault/);
    await userEvent.type(input, 'Hello?');
    await userEvent.click(screen.getByRole('button', { name: 'Send' }));

    await waitFor(() => {
      expect(screen.getByText(/Ollama unreachable/)).toBeInTheDocument();
    });
  });

  it('does not submit when the input is empty', async () => {
    render(<AIPanel onClose={onClose} onOpenNote={onOpenNote} vaultPath="/vault" />);

    const submitButton = screen.getByRole('button', { name: 'Send' });
    expect(submitButton).toBeDisabled();

    await userEvent.click(submitButton);
    expect(window.go.main.AIService.Ask).not.toHaveBeenCalled();
  });

  it('shows source links that open the referenced note', async () => {
    askResponse = {
      answer: 'Here is the source.',
      sources: [{ id: '1', path: '10-notes/source.md', title: 'Source Note' }],
      confidence: 'medium',
    };

    render(<AIPanel onClose={onClose} onOpenNote={onOpenNote} vaultPath="/vault" />);

    const input = screen.getByPlaceholderText(/Ask about your vault/);
    await userEvent.type(input, 'Show source');
    await userEvent.click(screen.getByRole('button', { name: 'Send' }));

    await waitFor(() => {
      expect(screen.getByText('Source Note')).toBeInTheDocument();
    });

    await userEvent.click(screen.getByText('Source Note'));
    expect(onOpenNote).toHaveBeenCalledWith('10-notes/source.md');
  });
});
