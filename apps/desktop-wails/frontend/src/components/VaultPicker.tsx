import { useState, useCallback } from 'react';
import { FolderOpen, Plus, HardDrive } from './Icons';

interface Props {
  onVaultOpened: () => void;
}

export default function VaultPicker({ onVaultOpened }: Props) {
  const [error, setError] = useState('');
  const [isCreating, setIsCreating] = useState(false);

  const handleOpenVault = useCallback(async () => {
    try {
      setError('');
      const path = await window.go.main.VaultService.SelectFolder();
      if (!path) return;

      if (await window.go.main.VaultService.IsVault(path)) {
        await window.go.main.VaultService.OpenVault(path);
      } else {
        setError(`'${path}' is not an AgentVault. Use "Create New Vault" to initialize it.`);
        return;
      }
      onVaultOpened();
    } catch (err: any) {
      setError(err.message || 'Failed to open vault');
    }
  }, [onVaultOpened]);

  const handleCreateVault = useCallback(async () => {
    try {
      setError('');
      const path = await window.go.main.VaultService.SelectFolder();
      if (!path) return;

      if (await window.go.main.VaultService.IsVault(path)) {
        setError(`'${path}' is already an AgentVault.`);
        return;
      }

      setIsCreating(true);
      await window.go.main.VaultService.InitVault(path);
      onVaultOpened();
    } catch (err: any) {
      setError(err.message || 'Failed to create vault');
    } finally {
      setIsCreating(false);
    }
  }, [onVaultOpened]);

  return (
    <div className="flex items-center justify-center h-screen bg-[var(--bg-primary)]">
      <div className="w-[480px]">
        <div className="text-center mb-10">
          <div className="flex items-center justify-center mb-4">
            <HardDrive className="w-10 h-10 text-[var(--accent)]" />
          </div>
          <h1 className="text-2xl font-semibold text-[var(--text-primary)] mb-2">
            AgentVault
          </h1>
          <p className="text-sm text-[var(--text-muted)]">
            Your notes, decisions, docs, and research — structured for humans, searchable by agents
          </p>
        </div>

        <div className="space-y-3">
          <button
            onClick={handleOpenVault}
            className="w-full flex items-center gap-3 px-4 py-3.5 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border)] hover:bg-[var(--bg-hover)] hover:border-[var(--accent)] transition-all text-left group"
          >
            <FolderOpen className="w-5 h-5 text-[var(--text-muted)] group-hover:text-[var(--accent)]" />
            <div>
              <div className="text-sm font-medium text-[var(--text-primary)]">Open Existing Vault</div>
              <div className="text-xs text-[var(--text-muted)]">Select an AgentVault folder</div>
            </div>
          </button>

          <button
            onClick={handleCreateVault}
            disabled={isCreating}
            className="w-full flex items-center gap-3 px-4 py-3.5 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border)] hover:bg-[var(--bg-hover)] hover:border-[var(--accent)] transition-all text-left group disabled:opacity-50"
          >
            <Plus className="w-5 h-5 text-[var(--text-muted)] group-hover:text-[var(--accent)]" />
            <div>
              <div className="text-sm font-medium text-[var(--text-primary)]">
                {isCreating ? 'Creating...' : 'Create New Vault'}
              </div>
              <div className="text-xs text-[var(--text-muted)]">Initialize a new AgentVault in any folder</div>
            </div>
          </button>
        </div>

        {error && (
          <div className="mt-4 px-4 py-3 rounded-lg bg-red-500/10 border border-red-500/20 text-sm text-red-400">
            {error}
          </div>
        )}

        <div className="mt-8 text-center text-xs text-[var(--text-muted)]">
          Local-first AI knowledge operating system
        </div>
      </div>
    </div>
  );
}
