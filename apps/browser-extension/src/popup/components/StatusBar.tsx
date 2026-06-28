import type { VaultStatus } from '@agentvault/contract';
import type { ClassifiedError } from '@shared/types';

interface StatusBarProps {
  connected: boolean;
  serverUrl: string;
  vault?: VaultStatus | null;
  lastError?: ClassifiedError | null;
}

export function StatusBar({ connected, serverUrl, vault, lastError }: StatusBarProps) {
  const dotColor = connected ? '#22c55e' : '#ef4444';
  const textColor = connected ? '#22c55e' : '#ef4444';
  const glow = connected ? '0 0 6px rgba(34,197,94,0.4)' : '0 0 6px rgba(239,68,68,0.4)';

  const vaultReady = connected && vault?.isVault;
  const noteCount = vaultReady ? vault.noteCount : undefined;

  let errorColor = '#ef4444';
  if (lastError?.kind === 'auth') errorColor = '#f59e0b';
  if (lastError?.kind === 'network') errorColor = '#f59e0b';

  return (
    <div style={{ padding: '8px 14px', background: '#1a1d27', borderBottom: '1px solid #2a2d3a' }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', minWidth: 0 }}>
          <div
            style={{
              width: '8px',
              height: '8px',
              borderRadius: '50%',
              background: dotColor,
              boxShadow: glow,
              flexShrink: 0,
            }}
          />
          <span
            style={{
              fontSize: '12px',
              color: textColor,
              fontWeight: 500,
              whiteSpace: 'nowrap',
            }}
          >
            {connected ? 'Connected' : 'Disconnected'}
            {noteCount !== undefined && ` • ${noteCount} note${noteCount === 1 ? '' : 's'}`}
          </span>
          {connected && vault && !vault.isVault && (
            <span style={{ fontSize: '11px', color: '#f59e0b', fontWeight: 500, whiteSpace: 'nowrap' }}>
              • Not a vault
            </span>
          )}
        </div>
        <span
          style={{
            fontSize: '11px',
            color: '#6b7280',
            fontFamily: 'monospace',
            marginLeft: '8px',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}
          title={serverUrl}
        >
          {serverUrl}
        </span>
      </div>
      {lastError && (
        <div
          style={{
            marginTop: '6px',
            padding: '6px 10px',
            background: 'rgba(239,68,68,0.08)',
            border: `1px solid ${errorColor}33`,
            borderRadius: '6px',
            color: errorColor,
            fontSize: '11px',
            lineHeight: '1.4',
          }}
        >
          <strong style={{ textTransform: 'capitalize' }}>{lastError.kind}</strong>
          {lastError.message && `: ${lastError.message}`}
        </div>
      )}
    </div>
  );
}
