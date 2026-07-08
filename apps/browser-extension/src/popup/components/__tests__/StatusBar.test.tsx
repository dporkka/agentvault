// @vitest-environment jsdom
import { describe, it, expect, beforeEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import { StatusBar } from '../StatusBar';

describe('StatusBar', () => {
  beforeEach(() => cleanup());
  it('renders connected state', () => {
    render(<StatusBar connected serverUrl="http://127.0.0.1:47321" />);
    expect(screen.getByText('Connected')).toBeInTheDocument();
    expect(screen.getByText('http://127.0.0.1:47321')).toBeInTheDocument();
  });

  it('renders disconnected state', () => {
    render(<StatusBar connected={false} serverUrl="http://127.0.0.1:47321" />);
    expect(screen.getByText('Disconnected')).toBeInTheDocument();
  });

  it('shows note count when connected to a vault', () => {
    render(<StatusBar connected serverUrl="http://127.0.0.1:47321" vault={{ isVault: true, noteCount: 5, path: '/vault', version: '1.0.0' }} />);
    expect(screen.getByText(/5 notes/)).toBeInTheDocument();
  });

  it('shows singular note count', () => {
    render(<StatusBar connected serverUrl="http://127.0.0.1:47321" vault={{ isVault: true, noteCount: 1, path: '/vault', version: '1.0.0' }} />);
    expect(screen.getByText(/1 note/)).toBeInTheDocument();
    expect(screen.queryByText(/1 notes/)).not.toBeInTheDocument();
  });

  it('warns when the server is not a vault', () => {
    render(<StatusBar connected serverUrl="http://127.0.0.1:47321" vault={{ isVault: false, noteCount: 0, path: '/vault', version: '1.0.0' }} />);
    expect(screen.getByText(/Not a vault/)).toBeInTheDocument();
  });

  it('renders a warning banner for auth errors', () => {
    render(
      <StatusBar
        connected={false}
        serverUrl="http://127.0.0.1:47321"
        lastError={{ kind: 'auth', message: 'Token expired', recoverable: true }}
      />,
    );
    const banner = screen.getByText(/Token expired/);
    expect(banner).toBeInTheDocument();
    expect(banner.closest('.banner-warning')).toBeInTheDocument();
  });

  it('renders an error banner for client errors', () => {
    render(
      <StatusBar
        connected={false}
        serverUrl="http://127.0.0.1:47321"
        lastError={{ kind: 'client', message: 'Bad input', recoverable: false }}
      />,
    );
    const banner = screen.getByText(/Bad input/);
    expect(banner).toBeInTheDocument();
    expect(banner.closest('.banner-error')).toBeInTheDocument();
  });
});
