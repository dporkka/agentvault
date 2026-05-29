import { useState, useEffect, useCallback } from 'react';
import Layout from './components/Layout';
import VaultPicker from './components/VaultPicker';
import type { VaultStatus } from './types';

function App() {
  const [vaultStatus, setVaultStatus] = useState<VaultStatus | null>(null);
  const [loading, setLoading] = useState(true);

  const checkVault = useCallback(async () => {
    try {
      const status = await window.go.main.VaultService.GetStatus();
      setVaultStatus(status);
    } catch (err) {
      console.error('Failed to get vault status:', err);
      setVaultStatus({ path: '', isOpen: false, noteCount: 0, version: '0.1.0' });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    checkVault();
  }, [checkVault]);

  const handleVaultOpened = useCallback(async () => {
    const status = await window.go.main.VaultService.GetStatus();
    setVaultStatus(status);
  }, []);

  if (loading) {
    return (
      <div className="flex items-center justify-center h-screen bg-[var(--bg-primary)]">
        <div className="text-[var(--text-muted)] text-sm">Loading AgentVault...</div>
      </div>
    );
  }

  if (!vaultStatus?.isOpen) {
    return <VaultPicker onVaultOpened={handleVaultOpened} />;
  }

  return <Layout vaultStatus={vaultStatus} onVaultChanged={handleVaultOpened} />;
}

export default App;
