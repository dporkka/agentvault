import React from 'react';
import type { VaultStatus as VaultStatusType } from '@agentvault/contract';

interface VaultStatusProps {
  status: VaultStatusType | null;
  connected: boolean;
  loading: boolean;
}

const VaultStatus: React.FC<VaultStatusProps> = ({ status, connected, loading }) => {
  if (loading) {
    return (
      <div className="flex items-center gap-2 text-sm text-vault-text-muted animate-pulse">
        <div className="w-2 h-2 rounded-full bg-vault-text-muted" />
        Connecting...
      </div>
    );
  }

  if (!connected) {
    return (
      <div className="flex items-center gap-2 text-sm">
        <span className="relative flex h-2 w-2">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-red-400 opacity-75" />
          <span className="relative inline-flex rounded-full h-2 w-2 bg-red-500" />
        </span>
        <span className="text-vault-error">Disconnected</span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-3">
      <div className="flex items-center gap-2">
        <span className="relative flex h-2 w-2">
          <span className="animate-pulse-dot absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
          <span className="relative inline-flex rounded-full h-2 w-2 bg-vault-success" />
        </span>
        <span className="text-sm text-vault-success font-medium">Connected</span>
      </div>
      {status && (
        <>
          <span className="text-vault-border">|</span>
          <span className="text-sm text-vault-text-secondary truncate max-w-[200px]" title={status.path}>
            {status.path}
          </span>
          <span className="text-vault-border">|</span>
          <span className="text-sm text-vault-text-secondary">
            {status.noteCount.toLocaleString()} notes
          </span>
        </>
      )}
    </div>
  );
};

export default VaultStatus;
