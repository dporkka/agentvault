import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor, act } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import AskPanel from '../AskPanel';

const mockAsk = vi.hoisted(() => vi.fn());

vi.mock('@/api/client', () => ({
  api: {
    ask: mockAsk,
  },
}));

async function submitForm(container: HTMLElement) {
  const form = container.querySelector('form');
  if (!form) throw new Error('Form not found');
  await act(async () => {
    form.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }));
  });
}

describe('AskPanel', () => {
  beforeEach(() => {
    mockAsk.mockReset();
  });

  it('renders example questions when there are no messages', () => {
    render(
      <MemoryRouter>
        <AskPanel />
      </MemoryRouter>,
    );

    expect(screen.getByText('What are the recent decisions made?')).toBeInTheDocument();
  });

  it('sends a question and displays the answer', async () => {
    const user = userEvent.setup();
    mockAsk.mockResolvedValueOnce({
      answer: 'The answer is 42.',
      sources: [{ id: 'note-1', path: 'notes/note-1.md', title: 'Source Note' }],
      confidence: 'high',
    });

    const { container } = render(
      <MemoryRouter>
        <AskPanel />
      </MemoryRouter>,
    );

    const input = screen.getByPlaceholderText('Ask a question...');
    await user.type(input, 'What is the answer?');
    await submitForm(container);

    await waitFor(() => expect(mockAsk).toHaveBeenCalledWith({ question: 'What is the answer?' }));
    expect(await screen.findByText('The answer is 42.')).toBeInTheDocument();
    expect(screen.getByText('Confidence: high')).toBeInTheDocument();
  });

  it('removes the user message when the request fails', async () => {
    const user = userEvent.setup();
    mockAsk.mockRejectedValueOnce(new Error('Ask failed'));

    const { container } = render(
      <MemoryRouter>
        <AskPanel />
      </MemoryRouter>,
    );

    await user.type(screen.getByPlaceholderText('Ask a question...'), 'Why?');
    await submitForm(container);

    expect(await screen.findByText('Ask failed')).toBeInTheDocument();
    expect(screen.queryByText('Why?')).not.toBeInTheDocument();
  });

  it('does not submit empty questions', async () => {
    const { container } = render(
      <MemoryRouter>
        <AskPanel />
      </MemoryRouter>,
    );

    await submitForm(container);
    expect(mockAsk).not.toHaveBeenCalled();
  });

  it('fills the input when an example question is clicked', async () => {
    const user = userEvent.setup();
    render(
      <MemoryRouter>
        <AskPanel />
      </MemoryRouter>,
    );

    await user.click(screen.getByText('What tasks are pending?'));
    expect(screen.getByDisplayValue('What tasks are pending?')).toBeInTheDocument();
  });
});
