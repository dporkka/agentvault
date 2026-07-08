import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import ConnectionBadge from '../ConnectionBadge';
import { useConnection } from '../../hooks/useConnection';

jest.mock('../../hooks/useConnection', () => ({
  useConnection: jest.fn(),
}));

describe('ConnectionBadge', () => {
  function render() {
    let renderer: TestRenderer.ReactTestRenderer;
    act(() => {
      renderer = TestRenderer.create(<ConnectionBadge />);
    });
    return renderer!;
  }

  it('renders the online state', () => {
    (useConnection as jest.Mock).mockReturnValue({ status: 'online' });
    const tree = render().toJSON();
    expect(JSON.stringify(tree)).toContain('Connected');
  });

  it('renders the offline state', () => {
    (useConnection as jest.Mock).mockReturnValue({ status: 'offline' });
    const tree = render().toJSON();
    expect(JSON.stringify(tree)).toContain('Offline');
  });

  it('renders the checking state', () => {
    (useConnection as jest.Mock).mockReturnValue({ status: 'checking' });
    const tree = render().toJSON();
    expect(JSON.stringify(tree)).toContain('...');
  });
});
