import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/react';
import VaultStatus from '../VaultStatus';
import type { VaultStatus as VaultStatusType } from '@agentvault/contract';

const status: VaultStatusType = {
  path: '/vault',
  isVault: true,
  noteCount: 42,
  version: '1.0.0',
};

describe('VaultStatus', () => {
  it('renders the loading state', () => {
    render(<VaultStatus status={null} connected={false} loading={true} />);
    expect(screen.getByText('Connecting...')).toBeInTheDocument();
  });

  it('renders the disconnected state', () => {
    render(<VaultStatus status={null} connected={false} loading={false} />);
    expect(screen.getByText('Disconnected')).toBeInTheDocument();
  });

  it('renders connected with vault details', () => {
    render(<VaultStatus status={status} connected={true} loading={false} authenticated={true} />);
    expect(screen.getByText('Connected')).toBeInTheDocument();
    expect(screen.getByText('/vault')).toBeInTheDocument();
    expect(screen.getByText('42 notes')).toBeInTheDocument();
  });

  it('shows not authenticated when authenticated is false', () => {
    render(<VaultStatus status={status} connected={true} loading={false} authenticated={false} />);
    expect(screen.getByText('Not authenticated')).toBeInTheDocument();
  });

  it('formats large note counts', () => {
    render(
      <VaultStatus
        status={{ ...status, noteCount: 1234 }}
        connected={true}
        loading={false}
        authenticated={true}
      />,
    );
    expect(screen.getByText('1,234 notes')).toBeInTheDocument();
  });
});
