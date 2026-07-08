import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import VaultPicker from '../VaultPicker';

const RECENT_VAULTS_KEY = 'agentvault-recent-vaults';

describe('VaultPicker', () => {
  const onVaultOpened = vi.fn();
  let selectedFolder = '/home/user/vault';
  let isVault = true;

  beforeEach(() => {
    localStorage.clear();
    onVaultOpened.mockReset();
    selectedFolder = '/home/user/vault';
    isVault = true;

    (window as any).go = {
      main: {
        VaultService: {
          SelectFolder: vi.fn(() => Promise.resolve(selectedFolder)),
          IsVault: vi.fn(() => Promise.resolve(isVault)),
          OpenVault: vi.fn(() => Promise.resolve()),
          InitVault: vi.fn(() => Promise.resolve()),
        },
      },
    };
  });

  it('renders the vault picker heading', () => {
    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    expect(screen.getByRole('heading', { name: /AgentVault/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Open Existing Vault/i })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: /Create New Vault/i })).toBeInTheDocument();
  });

  it('opens an existing vault and notifies the parent', async () => {
    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    await userEvent.click(screen.getByRole('button', { name: /Open Existing Vault/i }));

    await waitFor(() => {
      expect(window.go.main.VaultService.OpenVault).toHaveBeenCalledWith('/home/user/vault');
    });
    expect(onVaultOpened).toHaveBeenCalledTimes(1);
  });

  it('shows a notice when the selected folder is not a vault', async () => {
    isVault = false;
    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    await userEvent.click(screen.getByRole('button', { name: /Open Existing Vault/i }));

    await waitFor(() => {
      expect(screen.getByText(/Selected folder is not an AgentVault/i)).toBeInTheDocument();
    });
    expect(onVaultOpened).not.toHaveBeenCalled();
  });

  it('creates a new vault and notifies the parent', async () => {
    isVault = false;
    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    await userEvent.click(screen.getByRole('button', { name: /Create New Vault/i }));

    await waitFor(() => {
      expect(window.go.main.VaultService.InitVault).toHaveBeenCalledWith('/home/user/vault');
    });
    expect(onVaultOpened).toHaveBeenCalledTimes(1);
  });

  it('shows a notice when creating in an existing vault', async () => {
    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    await userEvent.click(screen.getByRole('button', { name: /Create New Vault/i }));

    await waitFor(() => {
      expect(screen.getByText(/already an AgentVault/i)).toBeInTheDocument();
    });
    expect(window.go.main.VaultService.InitVault).not.toHaveBeenCalled();
  });

  it('loads and displays recent vaults from localStorage', () => {
    localStorage.setItem(RECENT_VAULTS_KEY, JSON.stringify(['/vault/one', '/vault/two']));

    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    expect(screen.getByText('/vault/one')).toBeInTheDocument();
    expect(screen.getByText('/vault/two')).toBeInTheDocument();
  });

  it('opens a recent vault from the list', async () => {
    localStorage.setItem(RECENT_VAULTS_KEY, JSON.stringify(['/vault/one']));

    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    await userEvent.click(screen.getByText('/vault/one'));

    await waitFor(() => {
      expect(window.go.main.VaultService.OpenVault).toHaveBeenCalledWith('/vault/one');
    });
    expect(onVaultOpened).toHaveBeenCalledTimes(1);
  });

  it('removes a stale recent vault when it is no longer a vault', async () => {
    isVault = false;
    localStorage.setItem(RECENT_VAULTS_KEY, JSON.stringify(['/vault/stale']));

    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    const staleButton = screen.getByText('/vault/stale');
    await userEvent.click(staleButton);

    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /\/vault\/stale/i })).not.toBeInTheDocument();
    });

    const stored = JSON.parse(localStorage.getItem(RECENT_VAULTS_KEY) || '[]');
    expect(stored).not.toContain('/vault/stale');
  });

  it('offers to initialize a non-vault folder after selecting it', async () => {
    isVault = false;
    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    await userEvent.click(screen.getByRole('button', { name: /Open Existing Vault/i }));

    await waitFor(() => {
      expect(screen.getByText(/Initialize it as a vault/i)).toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole('button', { name: /Create Vault Here/i }));

    await waitFor(() => {
      expect(window.go.main.VaultService.InitVault).toHaveBeenCalledWith('/home/user/vault');
    });
  });

  it('limits recent vaults to five entries', async () => {
    localStorage.setItem(
      RECENT_VAULTS_KEY,
      JSON.stringify(['/v/1', '/v/2', '/v/3', '/v/4', '/v/5']),
    );

    render(<VaultPicker onVaultOpened={onVaultOpened} />);

    await userEvent.click(screen.getByRole('button', { name: /Open Existing Vault/i }));

    await waitFor(() => {
      const stored = JSON.parse(localStorage.getItem(RECENT_VAULTS_KEY) || '[]');
      expect(stored).toHaveLength(5);
      expect(stored[0]).toBe('/home/user/vault');
    });
  });
});
