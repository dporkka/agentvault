import { useState, useCallback } from 'react';
import { FolderOpen, Plus, HardDrive, AlertCircle, Info, X, Loader2 } from './Icons';

type NoticeType = 'error' | 'info';
interface Notice {
  type: NoticeType;
  message: string;
}

interface Props {
  onVaultOpened: () => void;
}

export default function VaultPicker({ onVaultOpened }: Props) {
  const [notice, setNotice] = useState<Notice | null>(null);
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [selectedIsVault, setSelectedIsVault] = useState<boolean | null>(null);
  const [isOpening, setIsOpening] = useState(false);
  const [isCreating, setIsCreating] = useState(false);

  const clearNotice = useCallback(() => {
    setNotice(null);
  }, []);

  const handleOpenVault = useCallback(async () => {
    clearNotice();
    setSelectedPath(null);
    setSelectedIsVault(null);
    setIsOpening(true);
    try {
      const path = await window.go.main.VaultService.SelectFolder();
      if (!path) {
        setIsOpening(false);
        return;
      }

      const vault = await window.go.main.VaultService.IsVault(path);
      if (vault) {
        await window.go.main.VaultService.OpenVault(path);
        onVaultOpened();
      } else {
        setSelectedPath(path);
        setSelectedIsVault(false);
        setNotice({
          type: 'info',
          message: `Selected folder is not an AgentVault. Use "Create New Vault" to initialize it.`,
        });
      }
    } catch (err: any) {
      setNotice({
        type: 'error',
        message: err.message || 'Failed to open vault',
      });
    } finally {
      setIsOpening(false);
    }
  }, [onVaultOpened, clearNotice]);

  const handleCreateVault = useCallback(async (path?: string) => {
    clearNotice();
    setIsCreating(true);
    try {
      const target = path || await window.go.main.VaultService.SelectFolder();
      if (!target) {
        setIsCreating(false);
        return;
      }

      if (await window.go.main.VaultService.IsVault(target)) {
        setSelectedPath(target);
        setSelectedIsVault(true);
        setNotice({
          type: 'info',
          message: `This folder is already an AgentVault. Use "Open Existing Vault" to open it.`,
        });
        setIsCreating(false);
        return;
      }

      await window.go.main.VaultService.InitVault(target);
      onVaultOpened();
    } catch (err: any) {
      setNotice({
        type: 'error',
        message: err.message || 'Failed to create vault',
      });
    } finally {
      setIsCreating(false);
    }
  }, [onVaultOpened, clearNotice]);

  const canCreateHere = selectedPath && selectedIsVault === false;

  return (
    <div className="flex items-center justify-center h-screen bg-[var(--bg-primary)]">
      <div className="w-[520px]">
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
            disabled={isOpening || isCreating}
            className="w-full flex items-center gap-3 px-4 py-3.5 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border)] hover:bg-[var(--bg-hover)] hover:border-[var(--accent)] transition-all text-left group disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {isOpening ? (
              <Loader2 className="w-5 h-5 text-[var(--accent)] animate-spin" />
            ) : (
              <FolderOpen className="w-5 h-5 text-[var(--text-muted)] group-hover:text-[var(--accent)]" />
            )}
            <div>
              <div className="text-sm font-medium text-[var(--text-primary)]">
                {isOpening ? 'Opening...' : 'Open Existing Vault'}
              </div>
              <div className="text-xs text-[var(--text-muted)]">Select an AgentVault folder</div>
            </div>
          </button>

          <button
            onClick={() => handleCreateVault()}
            disabled={isCreating || isOpening}
            className="w-full flex items-center gap-3 px-4 py-3.5 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border)] hover:bg-[var(--bg-hover)] hover:border-[var(--accent)] transition-all text-left group disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {isCreating ? (
              <Loader2 className="w-5 h-5 text-[var(--accent)] animate-spin" />
            ) : (
              <Plus className="w-5 h-5 text-[var(--text-muted)] group-hover:text-[var(--accent)]" />
            )}
            <div>
              <div className="text-sm font-medium text-[var(--text-primary)]">
                {isCreating ? 'Creating...' : 'Create New Vault'}
              </div>
              <div className="text-xs text-[var(--text-muted)]">Initialize a new AgentVault in any folder</div>
            </div>
          </button>
        </div>

        {selectedPath && (
          <div className="mt-4 px-4 py-2.5 rounded-lg bg-[var(--bg-secondary)] border border-[var(--border)] text-xs">
            <span className="text-[var(--text-muted)]">Selected:</span>{' '}
            <span className="text-[var(--text-primary)] font-mono break-all">{selectedPath}</span>
            {selectedIsVault !== null && (
              <span className={`ml-2 px-1.5 py-0.5 rounded text-[10px] font-medium ${
                selectedIsVault
                  ? 'bg-green-500/10 text-green-400'
                  : 'bg-yellow-500/10 text-yellow-400'
              }`}>
                {selectedIsVault ? 'vault' : 'not a vault'}
              </span>
            )}
          </div>
        )}

        {canCreateHere && (
          <div className="mt-3 px-4 py-3 rounded-lg bg-[var(--accent)]/10 border border-[var(--accent)]/20 text-sm">
            <div className="flex items-start gap-2.5">
              <Info className="w-4 h-4 text-[var(--accent)] mt-0.5 flex-shrink-0" />
              <div className="flex-1">
                <p className="text-[var(--text-primary)]">
                  Want to use this folder? Initialize it as a vault.
                </p>
                <button
                  onClick={() => handleCreateVault(selectedPath)}
                  disabled={isCreating}
                  className="mt-2 btn-primary text-xs"
                >
                  {isCreating ? 'Initializing...' : 'Create Vault Here'}
                </button>
              </div>
            </div>
          </div>
        )}

        {notice && (
          <div className={`mt-4 px-4 py-3 rounded-lg border text-sm flex items-start gap-2.5 ${
            notice.type === 'error'
              ? 'bg-red-500/10 border-red-500/20 text-red-400'
              : 'bg-blue-500/10 border-blue-500/20 text-blue-400'
          }`}>
            {notice.type === 'error' ? (
              <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
            ) : (
              <Info className="w-4 h-4 mt-0.5 flex-shrink-0" />
            )}
            <span className="flex-1">{notice.message}</span>
            <button
              onClick={clearNotice}
              className="hover:opacity-70 flex-shrink-0"
              aria-label="Dismiss"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        )}

        <div className="mt-8 text-center text-xs text-[var(--text-muted)]">
          Local-first AI knowledge operating system
        </div>
      </div>
    </div>
  );
}
