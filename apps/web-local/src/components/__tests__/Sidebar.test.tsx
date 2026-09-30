import { describe, it, expect, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import { MemoryRouter } from 'react-router-dom';
import Sidebar from '../Sidebar';

describe('Sidebar', () => {
  it('includes Activity as a first-class navigation destination', () => {
    render(
      <MemoryRouter>
        <Sidebar collapsed={false} onToggle={vi.fn()} />
      </MemoryRouter>,
    );

    expect(screen.getByRole('link', { name: /Activity/i })).toHaveAttribute('href', '/activity');
  });
});
