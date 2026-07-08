import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import { EmptyState } from '../EmptyState';

describe('EmptyState', () => {
  it('renders the title', () => {
    render(<EmptyState title="Nothing here" />);
    expect(screen.getByText('Nothing here')).toBeInTheDocument();
  });

  it('renders an optional subtitle', () => {
    render(<EmptyState title="No results" subtitle="Try a different query" />);
    expect(screen.getByText('No results')).toBeInTheDocument();
    expect(screen.getByText('Try a different query')).toBeInTheDocument();
  });

  it('renders an optional icon', () => {
    render(<EmptyState title="No items" icon={<span data-testid="icon">icon</span>} />);
    expect(screen.getByTestId('icon')).toBeInTheDocument();
  });

  it('applies the provided className', () => {
    const { container } = render(<EmptyState title="No items" className="custom-class" />);
    expect(container.firstChild).toHaveClass('custom-class');
  });
});
