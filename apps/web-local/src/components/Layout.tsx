import React, { useState, useEffect } from 'react';
import { Outlet, useNavigate } from 'react-router-dom';
import Sidebar from './Sidebar';
import VaultStatus from './VaultStatus';
import ConnectionModal from './ConnectionModal';
import { api } from '@/api/client';
import type { VaultStatus as VaultStatusType } from '@agentvault/contract';

const POLL_INTERVAL = 10000; // 10 seconds

const Layout: React.FC = () => {
  const navigate = useNavigate();
  const [sidebarCollapsed, setSidebarCollapsed] = useState(true);
  const [desktopCollapsed, setDesktopCollapsed] = useState(false);
  const [vaultStatus, setVaultStatus] = useState<VaultStatusType | null>(null);
  const [connected, setConnected] = useState(false);
  const [authenticated, setAuthenticated] = useState<boolean | null>(null);
  const [checking, setChecking] = useState(true);
  const [showConnectionModal, setShowConnectionModal] = useState(false);

  const checkConnection = async () => {
    try {
      const health = await api.checkHealth();
      setConnected(true);
      try {
        const status = await api.getVaultStatus();
        setVaultStatus(status);
      } catch {
        setVaultStatus({
          path: health.vault,
          isVault: true,
          noteCount: 0,
          version: health.version,
        });
      }

      // Check whether the stored token is still valid (non-fatal).
      try {
        const verify = await api.verifyAuth();
        setAuthenticated(verify.tokenValid);
      } catch {
        setAuthenticated(null);
      }
    } catch {
      setConnected(false);
      setVaultStatus(null);
      setAuthenticated(null);
    } finally {
      setChecking(false);
    }
  };

  useEffect(() => {
    checkConnection();
    const interval = setInterval(checkConnection, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, []);

  // Listen for storage changes (settings updated in another tab)
  useEffect(() => {
    function onStorage() {
      checkConnection();
    }
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  // Show the connection modal on first launch if the server is reachable but
  // the user has not configured a valid token yet.
  useEffect(() => {
    if (!checking && connected && authenticated === false) {
      setShowConnectionModal(true);
    }
  }, [checking, connected, authenticated]);

  return (
    <div className="h-full flex bg-vault-bg-primary">
      {/* Sidebar */}
      <Sidebar
        collapsed={sidebarCollapsed}
        onToggle={() => {
          if (window.innerWidth >= 1024) {
            setDesktopCollapsed(!desktopCollapsed);
            setSidebarCollapsed(!desktopCollapsed);
          } else {
            setSidebarCollapsed(!sidebarCollapsed);
          }
        }}
      />

      {/* Main content */}
      <div className={`flex-1 flex flex-col min-w-0 transition-all ${desktopCollapsed ? '' : ''}`}>
        {/* Top bar */}
        <header className="h-14 border-b border-vault-border flex items-center justify-between px-4 lg:px-6 flex-shrink-0 bg-vault-bg-secondary/50 backdrop-blur-sm">
          {/* Mobile menu toggle */}
          <button
            onClick={() => setSidebarCollapsed(false)}
            className="lg:hidden flex items-center justify-center w-8 h-8 rounded-lg hover:bg-vault-bg-hover text-vault-text-secondary"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
            </svg>
          </button>

          {/* Connection status */}
          <button
            onClick={() => setShowConnectionModal(true)}
            className="outline-none focus-visible:ring-2 focus-visible:ring-vault-accent rounded"
          >
            <VaultStatus
              status={vaultStatus}
              connected={connected}
              loading={checking}
              authenticated={authenticated}
            />
          </button>

          {/* Right side: Create note button */}
          <button
            onClick={() => navigate('/')}
            className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium text-vault-accent bg-vault-accent-muted rounded-lg hover:bg-vault-accent/20 transition-colors"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
            </svg>
            Search
          </button>
        </header>

        {/* Page content */}
        <main className="flex-1 overflow-hidden">
          <Outlet />
        </main>
      </div>

      <ConnectionModal
        open={showConnectionModal}
        onClose={() => {
          setShowConnectionModal(false);
          checkConnection();
        }}
      />
    </div>
  );
}

export default Layout;
