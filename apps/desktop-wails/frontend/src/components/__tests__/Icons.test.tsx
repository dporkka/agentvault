import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import {
  SearchIcon,
  FolderOpen,
  Plus,
  CheckCircle,
  AlertTriangle,
  Loader2,
  SettingsIcon,
} from '../Icons';

describe('Icons', () => {
  it.each([
    { Icon: SearchIcon, name: 'SearchIcon' },
    { Icon: FolderOpen, name: 'FolderOpen' },
    { Icon: Plus, name: 'Plus' },
    { Icon: CheckCircle, name: 'CheckCircle' },
    { Icon: AlertTriangle, name: 'AlertTriangle' },
    { Icon: Loader2, name: 'Loader2' },
    { Icon: SettingsIcon, name: 'SettingsIcon' },
  ])('$name renders an accessible SVG with the provided className', ({ Icon }) => {
    const { container } = render(<Icon className="w-5 h-5" />);
    const svg = container.querySelector('svg');
    expect(svg).toBeInTheDocument();
    expect(svg).toHaveAttribute('viewBox', '0 0 24 24');
    expect(svg).toHaveClass('w-5 h-5');
  });

  it('renders a title element when the title prop is provided', () => {
    const { container } = render(<CheckCircle className="w-5 h-5" title="Completed" />);
    const title = container.querySelector('title');
    expect(title).toHaveTextContent('Completed');
  });

  it('does not render a title element when the title prop is omitted', () => {
    const { container } = render(<CheckCircle className="w-5 h-5" />);
    expect(container.querySelector('title')).not.toBeInTheDocument();
  });
});
