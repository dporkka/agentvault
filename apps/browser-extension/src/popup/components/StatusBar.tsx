import type { VaultStatus } from '@agentvault/contract';
import type { ClassifiedError } from '@shared/types';

interface StatusBarProps {
  connected: boolean;
  serverUrl: string;
  vault?: VaultStatus | null;
  lastError?: ClassifiedError | null;
}

export function StatusBar({ connected, serverUrl, vault, lastError }: StatusBarProps) {
  const vaultReady = connected && vault?.isVault;
  const noteCount = vaultReady ? vault.noteCount : undefined;

  const isWarningError = lastError?.kind === 'auth' || lastError?.kind === 'network';

  return (
    <div className="status-bar">
      <div className="status-bar__row">
        <div className="status-bar__left">
          <div className={`status-dot ${connected ? 'status-dot--connected' : 'status-dot--disconnected'}`} />
          <span className={`status-bar__text ${connected ? 'status-bar__text--connected' : 'status-bar__text--disconnected'}`}>
            {connected ? 'Connected' : 'Disconnected'}
            {noteCount !== undefined && ` • ${noteCount} note${noteCount === 1 ? '' : 's'}`}
          </span>
          {connected && vault && !vault.isVault && (
            <span className="status-bar__text status-bar__text--warning">• Not a vault</span>
          )}
        </div>
        <span className="status-bar__url" title={serverUrl}>
          {serverUrl}
        </span>
      </div>
      {lastError && (
        <div className={`status-bar__error banner ${isWarningError ? 'banner-warning' : 'banner-error'}`}>
          <span className="status-bar__error-title">{lastError.kind}</span>
          {lastError.message && `: ${lastError.message}`}
        </div>
      )}
    </div>
  );
}
