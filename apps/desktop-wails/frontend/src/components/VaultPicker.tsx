import { useState, useCallback, useEffect } from 'react';
import { FolderOpen, Plus, HardDrive, AlertCircle, AlertTriangle, Info, X, Loader2, Clock, Folder, ChevronRight } from './Icons';

type NoticeType = 'error' | 'info';
interface Notice {
  type: NoticeType;
  message: string;
}

interface Props {
  onVaultOpened: () => void;
}

const RECENT_VAULTS_KEY = 'agentvault-recent-vaults';
const MAX_RECENT_VAULTS = 5;

function loadRecentVaults(): string[] {
  if (typeof window === 'undefined') return [];
  try {
    const raw = localStorage.getItem(RECENT_VAULTS_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed) && parsed.every((item) => typeof item === 'string')) {
      return parsed;
    }
  } catch {
    // Ignore malformed storage.
  }
  return [];
}

function saveRecentVaults(paths: string[]): void {
  if (typeof window === 'undefined') return;
  try {
    localStorage.setItem(RECENT_VAULTS_KEY, JSON.stringify(paths));
  } catch {
    // Ignore storage errors (e.g. private browsing).
  }
}

function addRecentVault(path: string): string[] {
  const current = loadRecentVaults();
  const next = [path, ...current.filter((p) => p !== path)].slice(0, MAX_RECENT_VAULTS);
  saveRecentVaults(next);
  return next;
}

export default function VaultPicker({ onVaultOpened }: Props) {
  const [notice, setNotice] = useState<Notice | null>(null);
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [selectedIsVault, setSelectedIsVault] = useState<boolean | null>(null);
  const [isOpening, setIsOpening] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [recentVaults, setRecentVaults] = useState<string[]>([]);

  useEffect(() => {
    setRecentVaults(loadRecentVaults());
  }, []);

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
        setRecentVaults(addRecentVault(path));
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
      setRecentVaults(addRecentVault(target));
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

  const openRecentVault = useCallback(async (path: string) => {
    clearNotice();
    setSelectedPath(path);
    setSelectedIsVault(null);
    setIsOpening(true);
    try {
      const vault = await window.go.main.VaultService.IsVault(path);
      if (vault) {
        await window.go.main.VaultService.OpenVault(path);
        setRecentVaults(addRecentVault(path));
        onVaultOpened();
      } else {
        setSelectedIsVault(false);
        setNotice({
          type: 'info',
          message: `This folder is no longer an AgentVault. Use "Create New Vault" to initialize it.`,
        });
        const trimmed = recentVaults.filter((p) => p !== path);
        setRecentVaults(trimmed);
        saveRecentVaults(trimmed);
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

  const canCreateHere = selectedPath && selectedIsVault === false;
  const isBusy = isOpening || isCreating;

  return (
    <div className="flex items-center justify-center h-screen bg-bg-primary px-6">
      <div className="w-full max-w-[520px]">
        <div className="text-center mb-10">
          <div className="flex items-center justify-center mb-5">
            <div className="relative">
              <div className="absolute inset-0 bg-accent/20 blur-2xl rounded-full" aria-hidden="true" />
              <HardDrive className="relative w-12 h-12 text-accent" />
            </div>
          </div>
          <h1 className="text-4xl font-bold tracking-tight text-text-primary mb-3">
            <span className="bg-gradient-to-r from-accent to-indigo-400 bg-clip-text text-transparent">
              AgentVault
            </span>
          </h1>
          <p className="text-base text-text-secondary">
            Your notes, decisions, docs, and research — structured for humans, searchable by agents
          </p>
        </div>

        {recentVaults.length > 0 && (
          <div className="mb-6">
            <div className="flex items-center gap-2 mb-3 text-text-muted">
              <Clock className="w-4 h-4" />
              <span className="text-xs font-semibold uppercase tracking-wider">Recent vaults</span>
            </div>
            <div className="space-y-2">
              {recentVaults.map((path) => (
                <button
                  key={path}
                  onClick={() => openRecentVault(path)}
                  disabled={isBusy}
                  className="w-full flex items-center gap-3 px-4 py-3 rounded-lg bg-bg-secondary border border-border hover:bg-bg-hover hover:border-accent transition-all text-left group disabled:opacity-60 disabled:cursor-not-allowed"
                >
                  <Folder className="w-5 h-5 text-text-muted group-hover:text-accent flex-shrink-0" />
                  <span className="flex-1 min-w-0 text-sm text-text-primary truncate font-mono">
                    {path}
                  </span>
                  <ChevronRight className="w-4 h-4 text-text-muted group-hover:text-accent flex-shrink-0 opacity-0 group-hover:opacity-100 transition-opacity" />
                </button>
              ))}
            </div>
          </div>
        )}

        <div className="space-y-3">
          <button
            onClick={handleOpenVault}
            disabled={isBusy}
            className="w-full flex items-center gap-3 px-4 py-3.5 rounded-lg bg-bg-secondary border border-border hover:bg-bg-hover hover:border-accent transition-all text-left group disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {isOpening ? (
              <Loader2 className="w-5 h-5 text-accent animate-spin" />
            ) : (
              <FolderOpen className="w-5 h-5 text-text-muted group-hover:text-accent" />
            )}
            <div>
              <div className="text-sm font-medium text-text-primary">
                {isOpening ? 'Opening vault…' : 'Open Existing Vault'}
              </div>
              <div className="text-xs text-text-muted">
                {isOpening ? 'Reading folder contents' : 'Select an AgentVault folder'}
              </div>
            </div>
          </button>

          <button
            onClick={() => handleCreateVault()}
            disabled={isBusy}
            className="w-full flex items-center gap-3 px-4 py-3.5 rounded-lg bg-bg-secondary border border-border hover:bg-bg-hover hover:border-accent transition-all text-left group disabled:opacity-60 disabled:cursor-not-allowed"
          >
            {isCreating ? (
              <Loader2 className="w-5 h-5 text-accent animate-spin" />
            ) : (
              <Plus className="w-5 h-5 text-text-muted group-hover:text-accent" />
            )}
            <div>
              <div className="text-sm font-medium text-text-primary">
                {isCreating ? 'Creating vault…' : 'Create New Vault'}
              </div>
              <div className="text-xs text-text-muted">
                {isCreating ? 'Initializing files' : 'Initialize a new AgentVault in any folder'}
              </div>
            </div>
          </button>
        </div>

        {selectedPath && (
          <div className="mt-4 px-4 py-2.5 rounded-lg bg-bg-secondary border border-border text-xs">
            <span className="text-text-muted">Selected:</span>{' '}
            <span className="text-text-primary font-mono break-all">{selectedPath}</span>
            {selectedIsVault !== null && (
              <span className={`ml-2 px-1.5 py-0.5 rounded text-[10px] font-medium ${
                selectedIsVault
                  ? 'bg-success/10 text-success'
                  : 'bg-warning/10 text-warning'
              }`}>
                {selectedIsVault ? 'vault' : 'not a vault'}
              </span>
            )}
          </div>
        )}

        {canCreateHere && (
          <div className="mt-3 px-4 py-3 rounded-lg bg-accent/10 border border-accent/20 text-sm">
            <div className="flex items-start gap-2.5">
              <Info className="w-4 h-4 text-accent mt-0.5 flex-shrink-0" />
              <div className="flex-1">
                <p className="text-text-primary">
                  Want to use this folder? Initialize it as a vault.
                </p>
                <button
                  onClick={() => handleCreateVault(selectedPath)}
                  disabled={isCreating}
                  className="mt-2 btn-primary text-xs"
                >
                  {isCreating ? 'Initializing…' : 'Create Vault Here'}
                </button>
              </div>
            </div>
          </div>
        )}

        {notice && (
          <div className={`mt-4 px-4 py-3 rounded-lg border text-sm flex items-start gap-2.5 ${
            notice.type === 'error'
              ? 'bg-error/10 border-error/20 text-error'
              : 'bg-warning/10 border-warning/20 text-warning'
          }`}>
            {notice.type === 'error' ? (
              <AlertCircle className="w-4 h-4 mt-0.5 flex-shrink-0" />
            ) : (
              <AlertTriangle className="w-4 h-4 mt-0.5 flex-shrink-0" />
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

        <div className="mt-8 text-center text-xs text-text-muted">
          Local-first AI knowledge operating system
        </div>
      </div>
    </div>
  );
}
