import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import EmptyState from '../EmptyState';
import { SearchIcon } from '../Icons';

describe('EmptyState', () => {
  it('renders the icon and title', () => {
    render(<EmptyState icon={<SearchIcon className="w-10 h-10" />} title="No results" />);

    expect(screen.getByRole('heading', { name: 'No results' })).toBeInTheDocument();
    expect(document.querySelector('svg')).toBeInTheDocument();
  });

  it('renders the description when provided', () => {
    render(
      <EmptyState
        icon={<SearchIcon className="w-10 h-10" />}
        title="No results"
        description="Try a different query"
      />,
    );

    expect(screen.getByText('Try a different query')).toBeInTheDocument();
  });

  it('does not render a description when omitted', () => {
    render(<EmptyState icon={<SearchIcon className="w-10 h-10" />} title="No results" />);

    expect(screen.queryByText(/Try a different query/)).not.toBeInTheDocument();
  });

  it('renders an action button and calls the handler', async () => {
    const handleClick = vi.fn();
    render(
      <EmptyState
        icon={<SearchIcon className="w-10 h-10" />}
        title="No results"
        action={{ label: 'Retry', onClick: handleClick }}
      />,
    );

    const button = screen.getByRole('button', { name: 'Retry' });
    expect(button).toBeInTheDocument();

    await userEvent.click(button);
    expect(handleClick).toHaveBeenCalledTimes(1);
  });
});
