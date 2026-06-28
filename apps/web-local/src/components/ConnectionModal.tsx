import React, { useState, useEffect } from 'react';
import { api } from '@/api/client';
import { DEFAULT_BASE_URL, type AuthVerifyResponse } from '@agentvault/contract';

interface ConnectionModalProps {
  open: boolean;
  onClose: () => void;
}

const ConnectionModal: React.FC<ConnectionModalProps> = ({ open, onClose }) => {
  const [serverUrl, setServerUrl] = useState(api.getBaseUrl());
  const [token, setToken] = useState(api.getToken());
  const [testing, setTesting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    if (open) {
      setServerUrl(api.getBaseUrl());
      setToken(api.getToken());
      setError(null);
      setSuccess(false);
    }
  }, [open]);

  const handleTestAndSave = async () => {
    setTesting(true);
    setError(null);
    setSuccess(false);

    api.setBaseUrl(serverUrl);
    api.setToken(token);

    try {
      const health = await api.checkHealth();
      if (!health || !health.status) {
        throw new Error('Server is reachable but returned an unexpected response.');
      }
    } catch (err) {
      setTesting(false);
      setError(err instanceof Error ? err.message : 'Could not connect to the server.');
      return;
    }

    let verify: AuthVerifyResponse | undefined;
    try {
      verify = await api.verifyAuth();
    } catch {
      // verifyAuth is optional; if it fails we still trust the token the user pasted.
    }

    setTesting(false);

    if (verify && !verify.tokenValid && token) {
      setError('The server rejected this token. Copy the token printed when you run agentvault serve.');
      return;
    }

    setSuccess(true);
    setTimeout(onClose, 600);
  };

  if (!open) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div className="w-full max-w-md bg-vault-bg-secondary border border-vault-border rounded-xl shadow-xl p-6">
        <h2 className="text-lg font-semibold text-vault-text-primary mb-1">Connect to AgentVault</h2>
        <p className="text-xs text-vault-text-muted mb-5">
          Enter the server URL and the token printed by <code className="text-vault-accent">agentvault serve</code>.
        </p>

        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-vault-text-secondary mb-1.5">Server URL</label>
            <input
              type="text"
              value={serverUrl}
              onChange={(e) => { setServerUrl(e.target.value); setError(null); setSuccess(false); }}
              placeholder={DEFAULT_BASE_URL}
              className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2.5 text-sm text-vault-text-primary placeholder-vault-text-muted focus:border-vault-accent focus:ring-1 focus:ring-vault-accent transition-colors outline-none font-mono"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-vault-text-secondary mb-1.5">Auth Token</label>
            <input
              type="password"
              value={token}
              onChange={(e) => { setToken(e.target.value); setError(null); setSuccess(false); }}
              placeholder="Paste your token here"
              className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2.5 text-sm text-vault-text-primary placeholder-vault-text-muted focus:border-vault-accent focus:ring-1 focus:ring-vault-accent transition-colors outline-none font-mono"
            />
          </div>

          {error && (
            <div className="flex items-start gap-2 p-3 rounded-lg text-sm bg-red-500/10 text-red-400">
              <svg className="w-4 h-4 flex-shrink-0 mt-0.5" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m9-.75a9 9 0 1 1-18 0 9 9 0 0 1 18 0Zm-9 3.75h.008v.008H12v-.008Z" />
              </svg>
              {error}
            </div>
          )}

          {success && (
            <div className="flex items-center gap-2 p-3 rounded-lg text-sm bg-emerald-500/10 text-emerald-400">
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75 11.25 15 15 9.75M21 12a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
              Connected successfully.
            </div>
          )}

          <div className="flex items-center gap-3 pt-2">
            <button
              onClick={handleTestAndSave}
              disabled={testing || !serverUrl.trim()}
              className="flex items-center gap-2 px-4 py-2 bg-vault-accent hover:bg-vault-accent-hover disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {testing && <div className="w-3.5 h-3.5 border-2 border-white border-t-transparent rounded-full animate-spin" />}
              Connect
            </button>
            <button
              onClick={onClose}
              className="px-4 py-2 text-sm text-vault-text-muted hover:text-vault-text-primary transition-colors"
            >
              Cancel
            </button>
          </div>
        </div>

        <div className="mt-5 pt-4 border-t border-vault-border text-xs text-vault-text-muted space-y-1.5">
          <p>
            <span className="text-vault-text-secondary font-medium">1.</span> Start the server:
          </p>
          <pre className="bg-vault-bg-tertiary rounded px-3 py-2 font-mono text-vault-accent overflow-x-auto">agentvault serve</pre>
          <p>
            <span className="text-vault-text-secondary font-medium">2.</span> Copy the token from the terminal output.
          </p>
          <p>
            <span className="text-vault-text-secondary font-medium">3.</span> Paste it above and click Connect.
          </p>
        </div>
      </div>
    </div>
  );
};

export default ConnectionModal;
