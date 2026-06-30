import { useState, useEffect, useCallback } from 'react';
import { HardDrive, Sparkles, CheckCircle, AlertTriangle, X, Loader2, Info, RefreshCw, ExternalLink } from './Icons';
import type { AIStatus, IndexingStatus, VaultStatus } from '../types';

interface Props {
  vaultPath: string;
}

const AI_PROVIDERS = ['ollama', 'openai', 'anthropic', 'openrouter', 'mock'];

const DEFAULT_URLS: Record<string, string> = {
  ollama: 'http://localhost:11434',
  openai: 'https://api.openai.com/v1',
  anthropic: 'https://api.anthropic.com/v1',
  openrouter: 'https://openrouter.ai/api/v1',
  mock: '',
};

const DEFAULT_MODELS: Record<string, string> = {
  ollama: 'llama3.1',
  openai: 'gpt-4o-mini',
  anthropic: 'claude-3-5-sonnet-20241022',
  openrouter: 'meta-llama/llama-3.1-70b',
  mock: '',
};

export default function SettingsView({ vaultPath }: Props) {
  const [aiStatus, setAiStatus] = useState<AIStatus | null>(null);
  const [aiEnabled, setAiEnabled] = useState(false);
  const [indexStatus, setIndexStatus] = useState<IndexingStatus>({ isIndexing: false, noteCount: 0 });
  const [vaultStatus, setVaultStatus] = useState<VaultStatus | null>(null);
  const [reindexing, setReindexing] = useState(false);
  const [provider, setProvider] = useState('ollama');
  const [baseUrl, setBaseUrl] = useState('http://localhost:11434');
  const [model, setModel] = useState('llama3.1');
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState('');
  const [error, setError] = useState('');

  const clearFeedback = useCallback(() => {
    setToast('');
    setError('');
  }, []);

  const loadStatus = useCallback(async () => {
    try {
      const enabled = await window.go.main.AIService.IsAIEnabled();
      setAiEnabled(enabled);
    } catch (err) {
      console.error('Failed to load AI enabled state:', err);
    }
    try {
      const status = await window.go.main.AIService.GetStatus();
      setAiStatus(status);
      if (status.provider) {
        setProvider(status.provider);
        setModel(status.model);
      }
    } catch (err: any) {
      console.error('Failed to load AI status:', err);
    }
    try {
      const status = await window.go.main.IndexService.GetStatus();
      setIndexStatus(status);
    } catch (err) {
      console.error('Failed to load index status:', err);
    }
    try {
      const status = await window.go.main.VaultService.GetStatus();
      setVaultStatus(status);
    } catch (err) {
      console.error('Failed to load vault status:', err);
    }
  }, []);

  useEffect(() => {
    loadStatus();
  }, [loadStatus]);

  const showToast = useCallback((message: string) => {
    setToast(message);
    setTimeout(() => setToast(''), 4000);
  }, []);

  const handleReindex = useCallback(async () => {
    clearFeedback();
    setReindexing(true);
    try {
      await window.go.main.IndexService.Index(true);
      const status = await window.go.main.IndexService.GetStatus();
      setIndexStatus(status);
      showToast('Vault reindexed successfully');
    } catch (err: any) {
      setError(err.message || 'Failed to reindex vault');
    } finally {
      setReindexing(false);
    }
  }, [clearFeedback, showToast]);

  const handleSaveAIConfig = useCallback(async () => {
    clearFeedback();
    setSaving(true);
    try {
      await window.go.main.AIService.SaveAIConfig(provider, baseUrl, model);
      await loadStatus();
      showToast('AI settings saved');
    } catch (err: any) {
      setError(err.message || 'Failed to save AI settings');
    } finally {
      setSaving(false);
    }
  }, [provider, baseUrl, model, clearFeedback, loadStatus, showToast]);

  const handleTestAI = useCallback(async () => {
    clearFeedback();
    setTesting(true);
    try {
      const status = await window.go.main.AIService.GetStatus();
      setAiStatus(status);
      if (status.enabled) {
        showToast(`AI reachable: ${status.provider} · ${status.model}`);
      } else {
        setError(status.error || 'AI is not reachable');
      }
    } catch (err: any) {
      setError(err.message || 'Failed to test AI connection');
    } finally {
      setTesting(false);
    }
  }, [clearFeedback, showToast]);

  const handleProviderChange = useCallback((next: string) => {
    setProvider(next);
    if (!baseUrl || DEFAULT_URLS[provider] === baseUrl) {
      setBaseUrl(DEFAULT_URLS[next] || '');
    }
    if (!model || DEFAULT_MODELS[provider] === model) {
      setModel(DEFAULT_MODELS[next] || '');
    }
  }, [baseUrl, model, provider]);

  const providerHelp: Record<string, string> = {
    ollama: 'Install Ollama, then run: ollama pull {model}',
    openai: 'Add your OpenAI API key to the vault config to enable chat.',
    anthropic: 'Add your Anthropic API key to the vault config to enable chat.',
    openrouter: 'Add your OpenRouter API key to the vault config to enable chat.',
    mock: 'Mock provider returns a static response for testing.',
  };

  return (
    <div className="flex flex-col h-full bg-[var(--bg-primary)] relative">
      {/* Header */}
      <div className="px-6 py-4 border-b border-[var(--border)] bg-[var(--bg-secondary)]">
        <h1 className="text-lg font-semibold text-[var(--text-primary)]">Settings</h1>
      </div>

      {/* Settings Content */}
      <div className="flex-1 overflow-auto p-6">
        <div className="max-w-2xl space-y-6">
          {/* Vault Info */}
          <section className="bg-[var(--bg-secondary)] rounded-lg border border-[var(--border)] p-4">
            <h2 className="text-sm font-medium text-[var(--text-primary)] mb-3 flex items-center gap-2">
              <HardDrive className="w-4 h-4 text-[var(--accent)]" />
              Vault
            </h2>
            <div className="space-y-2 text-sm">
              <div className="flex justify-between items-center">
                <span className="text-[var(--text-muted)]">Status</span>
                <span className="flex items-center gap-1.5 text-[var(--text-primary)]">
                  {vaultPath ? (
                    <>
                      <CheckCircle className="w-3.5 h-3.5 text-[var(--success)]" />
                      Vault ready
                    </>
                  ) : (
                    <>
                      <AlertTriangle className="w-3.5 h-3.5 text-yellow-400" />
                      No vault loaded
                    </>
                  )}
                </span>
              </div>
              <div className="flex justify-between">
                <span className="text-[var(--text-muted)]">Path</span>
                <span className="text-[var(--text-primary)] font-mono text-xs text-right max-w-[60%] break-all">{vaultPath || '-'}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-[var(--text-muted)]">Notes indexed</span>
                <span className="text-[var(--text-primary)]">{vaultStatus?.noteCount ?? indexStatus.noteCount}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-[var(--text-muted)]">Version</span>
                <span className="text-[var(--text-primary)]">0.1.0</span>
              </div>
            </div>
            <button
              onClick={handleReindex}
              disabled={reindexing}
              className="mt-3 btn-secondary text-xs flex items-center gap-1.5"
            >
              {reindexing ? (
                <Loader2 className="w-3.5 h-3.5 animate-spin" />
              ) : (
                <RefreshCw className="w-3.5 h-3.5" />
              )}
              {reindexing ? 'Reindexing...' : 'Force Reindex'}
            </button>
            <div className="mt-3 text-xs">
              <a
                href="https://github.com/agentvault/agentvault/releases"
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex items-center gap-1.5 text-[var(--accent)] hover:underline"
              >
                <ExternalLink className="w-3.5 h-3.5" />
                Check for updates
              </a>
            </div>
          </section>

          {/* AI Settings */}
          <section className="bg-[var(--bg-secondary)] rounded-lg border border-[var(--border)] p-4">
            <h2 className="text-sm font-medium text-[var(--text-primary)] mb-3 flex items-center gap-2">
              <Sparkles className="w-4 h-4 text-[var(--accent)]" />
              AI Provider
              {aiEnabled ? (
                <CheckCircle className="w-3.5 h-3.5 text-[var(--success)]" title="AI enabled" />
              ) : (
                <AlertTriangle className="w-3.5 h-3.5 text-yellow-400" title="AI not configured" />
              )}
            </h2>

            {aiStatus && (
              <div className={`mb-3 px-3 py-2 rounded-lg border text-xs ${
                aiStatus.enabled
                  ? 'bg-green-500/10 border-green-500/20 text-green-400'
                  : 'bg-yellow-500/10 border-yellow-500/20 text-yellow-400'
              }`}>
                <div className="flex items-center gap-2">
                  {aiStatus.enabled ? <CheckCircle className="w-3.5 h-3.5" /> : <AlertTriangle className="w-3.5 h-3.5" />}
                  <span className="font-medium">
                    {aiStatus.enabled ? `${aiStatus.provider} · ${aiStatus.model}` : 'AI not reachable'}
                  </span>
                </div>
                {aiStatus.error && (
                  <p className="mt-1 ml-5 text-[var(--text-muted)]">{aiStatus.error}</p>
                )}
              </div>
            )}

            <div className="space-y-3">
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">
                  Provider
                </label>
                <select
                  value={provider}
                  onChange={(e) => handleProviderChange(e.target.value)}
                  className="w-full input"
                >
                  {AI_PROVIDERS.map(p => (
                    <option key={p} value={p}>{p}</option>
                  ))}
                </select>
              </div>

              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">
                  Base URL
                </label>
                <input
                  type="text"
                  value={baseUrl}
                  onChange={(e) => setBaseUrl(e.target.value)}
                  className="w-full input"
                  placeholder={DEFAULT_URLS[provider] || 'https://api.example.com/v1'}
                />
              </div>

              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">
                  Chat Model
                </label>
                <input
                  type="text"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  className="w-full input"
                  placeholder={DEFAULT_MODELS[provider] || 'model-name'}
                />
              </div>

              <div className="text-xs text-[var(--text-muted)] flex items-start gap-2">
                <Info className="w-4 h-4 flex-shrink-0 mt-0.5" />
                <span>
                  {providerHelp[provider].replace('{model}', model || DEFAULT_MODELS[provider])}
                  {provider === 'ollama' && (
                    <> See <a href="https://ollama.com" target="_blank" rel="noopener noreferrer" className="text-[var(--accent)] hover:underline">ollama.com</a>.</>
                  )}
                </span>
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={handleSaveAIConfig}
                  disabled={saving}
                  className="btn-primary text-xs flex items-center gap-1.5"
                >
                  {saving ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : null}
                  {saving ? 'Saving...' : 'Save AI Settings'}
                </button>
                <button
                  onClick={handleTestAI}
                  disabled={testing}
                  className="btn-secondary text-xs flex items-center gap-1.5"
                >
                  {testing ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Sparkles className="w-3.5 h-3.5" />}
                  {testing ? 'Testing...' : 'Test Connection'}
                </button>
              </div>
            </div>
          </section>

          {/* Keyboard Shortcuts */}
          <section className="bg-[var(--bg-secondary)] rounded-lg border border-[var(--border)] p-4">
            <h2 className="text-sm font-medium text-[var(--text-primary)] mb-3">
              Keyboard Shortcuts
            </h2>
            <div className="space-y-1.5 text-sm">
              {[
                { key: 'Ctrl+K', desc: 'Command palette / search' },
                { key: '/', desc: 'Focus search bar' },
                { key: 'Ctrl+N', desc: 'New note' },
                { key: 'Ctrl+S', desc: 'Save note' },
                { key: 'Ctrl+B', desc: 'Toggle sidebar' },
                { key: 'Ctrl+J', desc: 'Toggle AI panel' },
                { key: 'Escape', desc: 'Close modal / panel' },
              ].map(({ key, desc }) => (
                <div key={key} className="flex justify-between items-center">
                  <span className="text-[var(--text-muted)] text-xs">{desc}</span>
                  <kbd className="px-1.5 py-0.5 rounded bg-[var(--bg-tertiary)] text-[var(--text-secondary)] text-[10px] font-mono">
                    {key}
                  </kbd>
                </div>
              ))}
            </div>
          </section>
        </div>
      </div>

      {/* Error banner */}
      {error && (
        <div className="absolute top-4 right-4 max-w-sm px-4 py-3 rounded-lg bg-red-500/10 border border-red-500/20 text-sm text-red-400 flex items-start gap-2.5 shadow-lg">
          <AlertTriangle className="w-4 h-4 mt-0.5 flex-shrink-0" />
          <span className="flex-1">{error}</span>
          <button onClick={() => setError('')} className="hover:opacity-70 flex-shrink-0">
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* Toast */}
      {toast && (
        <div className="absolute bottom-4 left-1/2 -translate-x-1/2 px-4 py-2 bg-[var(--accent)] text-white text-sm rounded-lg shadow-lg flex items-center gap-2">
          {toast}
          <button onClick={() => setToast('')} className="hover:opacity-70">
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}
    </div>
  );
}
