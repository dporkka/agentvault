import React, { useState, useEffect } from 'react';
import { api } from '@/api/client';
import { DEFAULT_BASE_URL, type HealthResponse } from '@agentvault/contract';

const SettingsPanel: React.FC = () => {
  const [serverUrl, setServerUrl] = useState(api.getBaseUrl());
  const [token, setToken] = useState(api.getToken());
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ success: boolean; message: string } | null>(null);
  const [vaultInfo, setVaultInfo] = useState<HealthResponse | null>(null);

  // Load saved values on mount
  useEffect(() => {
    setServerUrl(api.getBaseUrl());
    setToken(api.getToken());
  }, []);

  const handleSave = () => {
    api.setBaseUrl(serverUrl);
    api.setToken(token);
    setTestResult(null);
  };

  const handleTest = async () => {
    // Save current values first
    api.setBaseUrl(serverUrl);
    api.setToken(token);
    setTesting(true);
    setTestResult(null);

    try {
      const health = await api.checkHealth();
      setVaultInfo(health);
      setTestResult({
        success: true,
        message: `Connected! Server v${health.version} — Vault: ${health.vault}`,
      });
    } catch (err) {
      setVaultInfo(null);
      setTestResult({
        success: false,
        message: err instanceof Error ? err.message : 'Connection failed',
      });
    } finally {
      setTesting(false);
    }
  };

  const handleClear = () => {
    setServerUrl(DEFAULT_BASE_URL);
    setToken('');
    api.setBaseUrl(DEFAULT_BASE_URL);
    api.setToken('');
    setTestResult(null);
    setVaultInfo(null);
  };

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b border-vault-border px-6 py-4">
        <h1 className="text-lg font-semibold text-vault-text-primary">Settings</h1>
        <p className="text-xs text-vault-text-muted mt-1">Configure connection to your AgentVault server</p>
      </div>

      <div className="flex-1 overflow-y-auto px-6 py-6">
        <div className="max-w-lg space-y-6">
          {/* Server URL */}
          <div>
            <label className="block text-sm font-medium text-vault-text-secondary mb-1.5">
              Server URL
            </label>
            <input
              type="text"
              value={serverUrl}
              onChange={(e) => { setServerUrl(e.target.value); setTestResult(null); }}
              placeholder={DEFAULT_BASE_URL}
              className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2.5 text-sm text-vault-text-primary placeholder-vault-text-muted focus:border-vault-accent focus:ring-1 focus:ring-vault-accent transition-colors outline-none font-mono"
            />
            <p className="text-xs text-vault-text-muted mt-1.5">
              The URL where your AgentVault server is running
            </p>
          </div>

          {/* Auth Token */}
          <div>
            <label className="block text-sm font-medium text-vault-text-secondary mb-1.5">
              Auth Token
            </label>
            <input
              type="password"
              value={token}
              onChange={(e) => { setToken(e.target.value); setTestResult(null); }}
              placeholder="Your auth token"
              className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2.5 text-sm text-vault-text-primary placeholder-vault-text-muted focus:border-vault-accent focus:ring-1 focus:ring-vault-accent transition-colors outline-none font-mono"
            />
            <p className="text-xs text-vault-text-muted mt-1.5">
              The token printed when you run <code className="text-vault-accent">agentvault serve</code>. Required for write operations.
            </p>
          </div>

          {/* Test Result */}
          {testResult && (
            <div className={`flex items-start gap-2 p-3 rounded-lg text-sm ${
              testResult.success ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'
            }`}>
              <svg className="w-4 h-4 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
                {testResult.success ? (
                  <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
                ) : (
                  <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
                )}
              </svg>
              {testResult.message}
            </div>
          )}

          {/* Vault Info */}
          {vaultInfo && testResult?.success && (
            <div className="bg-vault-bg-tertiary border border-vault-border rounded-lg p-4 space-y-2 animate-fade-in">
              <h3 className="text-sm font-medium text-vault-text-primary">Vault Info</h3>
              <div className="grid grid-cols-2 gap-2 text-xs">
                <div>
                  <span className="text-vault-text-muted">Status</span>
                  <p className="text-vault-success font-medium">{vaultInfo.status}</p>
                </div>
                <div>
                  <span className="text-vault-text-muted">Version</span>
                  <p className="text-vault-text-primary font-medium font-mono">{vaultInfo.version}</p>
                </div>
                <div className="col-span-2">
                  <span className="text-vault-text-muted">Vault Path</span>
                  <p className="text-vault-text-primary font-medium font-mono truncate">{vaultInfo.vault}</p>
                </div>
              </div>
            </div>
          )}

          {/* Actions */}
          <div className="flex items-center gap-3 pt-2">
            <button
              onClick={handleTest}
              disabled={testing}
              className="flex items-center gap-2 px-4 py-2 bg-vault-accent hover:bg-vault-accent-hover disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {testing && <div className="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin" />}
              Test Connection
            </button>
            <button
              onClick={handleSave}
              className="px-4 py-2 border border-vault-border text-sm text-vault-text-secondary hover:bg-vault-bg-hover hover:text-vault-text-primary rounded-lg transition-colors"
            >
              Save Settings
            </button>
            <button
              onClick={handleClear}
              className="px-4 py-2 text-sm text-vault-text-muted hover:text-vault-error transition-colors"
            >
              Reset
            </button>
          </div>

          {/* Help */}
          <div className="border-t border-vault-border pt-4 mt-4">
            <h3 className="text-sm font-medium text-vault-text-primary mb-2">Getting Started</h3>
            <div className="bg-vault-bg-tertiary border border-vault-border rounded-lg p-4 space-y-2 text-xs text-vault-text-secondary">
              <p>
                <span className="text-vault-text-primary font-medium">1.</span> Start the AgentVault server:
              </p>
              <pre className="bg-vault-bg-primary rounded px-3 py-2 font-mono text-vault-accent overflow-x-auto">
                agentvault serve
              </pre>
              <p>
                <span className="text-vault-text-primary font-medium">2.</span> Copy the token printed in the terminal
              </p>
              <p>
                <span className="text-vault-text-primary font-medium">3.</span> Paste it above and click Test Connection
              </p>
              <p className="text-vault-text-muted pt-1">
                The server runs locally on port 47321 by default.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

export default SettingsPanel;
