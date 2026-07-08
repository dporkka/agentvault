import { useState, useEffect, useCallback } from 'react';
import { HardDrive, Sparkles, CheckCircle, AlertTriangle, X, Loader2, Info, RefreshCw, Server, Shield, Inbox, Copy, Check } from './Icons';
import { useTheme, type Theme } from '../hooks/useTheme';
import type { AIStatus, IndexingStatus, ServerStatus, CaptureInfo } from '../types';

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

type TabId = 'vault' | 'ai' | 'server' | 'appearance' | 'shortcuts';

interface Tab {
  id: TabId;
  label: string;
  icon: React.ReactNode;
}

function Sun({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  );
}

function Moon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
    </svg>
  );
}

function Monitor({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
    </svg>
  );
}

export default function SettingsView({ vaultPath }: Props) {
  const [activeTab, setActiveTab] = useState<TabId>('vault');
  const { theme, setTheme, resolved } = useTheme();

  const [aiStatus, setAiStatus] = useState<AIStatus | null>(null);
  const [aiEnabled, setAiEnabled] = useState(false);
  const [indexStatus, setIndexStatus] = useState<IndexingStatus>({ isIndexing: false, noteCount: 0 });
  const [reindexing, setReindexing] = useState(false);
  const [provider, setProvider] = useState('ollama');
  const [baseUrl, setBaseUrl] = useState('http://localhost:11434');
  const [model, setModel] = useState('llama3.1');
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);
  const [toast, setToast] = useState('');
  const [error, setError] = useState('');

  const [serverStatus, setServerStatus] = useState<ServerStatus | null>(null);
  const [serverToggling, setServerToggling] = useState(false);
  const [serverAddr, setServerAddr] = useState('127.0.0.1:47321');
  const [tokenCopied, setTokenCopied] = useState(false);
  const [authTesting, setAuthTesting] = useState(false);

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
      const status = await window.go.main.ServerService.GetServerStatus();
      setServerStatus(status);
      if (status.address) {
        setServerAddr(status.address);
      }
    } catch (err) {
      console.error('Failed to load server status:', err);
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

  const handleToggleServer = useCallback(async () => {
    clearFeedback();
    setServerToggling(true);
    try {
      if (serverStatus?.running) {
        await window.go.main.ServerService.StopServer();
        showToast('Local API server stopped');
      } else {
        await window.go.main.ServerService.StartServer(serverAddr);
        showToast(`Local API server started on ${serverAddr}`);
      }
      await loadStatus();
    } catch (err: any) {
      setError(err.message || 'Failed to toggle local API server');
    } finally {
      setServerToggling(false);
    }
  }, [clearFeedback, serverStatus?.running, serverAddr, loadStatus, showToast]);

  const handleCopyToken = useCallback(async () => {
    if (!serverStatus?.token) return;
    try {
      await navigator.clipboard.writeText(serverStatus.token);
      setTokenCopied(true);
      setTimeout(() => setTokenCopied(false), 2000);
    } catch {
      setError('Failed to copy token to clipboard');
    }
  }, [serverStatus?.token]);

  const handleTestAuth = useCallback(async () => {
    if (!serverStatus?.token) return;
    clearFeedback();
    setAuthTesting(true);
    try {
      const valid = await window.go.main.ServerService.IsAuthValid(serverStatus.token);
      if (valid) {
        showToast('Auth token is valid');
      } else {
        setError('Auth token is invalid');
      }
    } catch (err: any) {
      setError(err.message || 'Failed to validate auth token');
    } finally {
      setAuthTesting(false);
    }
  }, [clearFeedback, serverStatus?.token, showToast]);

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

  const tabs: Tab[] = [
    { id: 'vault', label: 'Vault', icon: <HardDrive className="w-4 h-4" /> },
    { id: 'ai', label: 'AI Provider', icon: <Sparkles className="w-4 h-4" /> },
    { id: 'server', label: 'Local API Server', icon: <Server className="w-4 h-4" /> },
    { id: 'appearance', label: 'Appearance', icon: resolved === 'dark' ? <Moon className="w-4 h-4" /> : <Sun className="w-4 h-4" /> },
    { id: 'shortcuts', label: 'Keyboard Shortcuts', icon: <Check className="w-4 h-4" /> },
  ];

  const shortcuts = [
    { key: 'Ctrl+K', desc: 'Command palette / search' },
    { key: '/', desc: 'Focus search bar' },
    { key: 'Ctrl+N', desc: 'New note' },
    { key: 'Ctrl+S', desc: 'Save note' },
    { key: 'Ctrl+B', desc: 'Toggle sidebar' },
    { key: 'Ctrl+J', desc: 'Toggle AI panel' },
    { key: 'Escape', desc: 'Close modal / panel' },
  ];

  const renderVaultTab = () => (
    <section className="bg-bg-secondary rounded-lg border border-border p-4">
      <h2 className="text-sm font-medium text-text-primary mb-3 flex items-center gap-2">
        <HardDrive className="w-4 h-4 text-accent" />
        Vault
      </h2>
      <div className="space-y-2 text-sm">
        <div className="flex justify-between">
          <span className="text-text-muted">Path</span>
          <span className="text-text-primary font-mono text-xs text-right max-w-[60%] break-all">{vaultPath}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-text-muted">Notes indexed</span>
          <span className="text-text-primary">{indexStatus.noteCount}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-text-muted">Version</span>
          <span className="text-text-primary">0.1.0</span>
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
    </section>
  );

  const renderServerTab = () => (
    <section className="bg-bg-secondary rounded-lg border border-border p-4">
      <h2 className="text-sm font-medium text-text-primary mb-3 flex items-center gap-2">
        <Server className="w-4 h-4 text-accent" />
        Local API Server
        {serverStatus?.running ? (
          <CheckCircle className="w-3.5 h-3.5 text-success" title="Server running" />
        ) : (
          <AlertTriangle className="w-3.5 h-3.5 text-warning" title="Server not running" />
        )}
      </h2>

      {serverStatus && (
        <div className={`mb-3 px-3 py-2 rounded-lg border text-xs ${
          serverStatus.running
            ? 'bg-success/10 border-success/20 text-success'
            : 'bg-warning/10 border-warning/20 text-warning'
        }`}>
          <div className="flex items-center gap-2">
            {serverStatus.running ? <CheckCircle className="w-3.5 h-3.5" /> : <AlertTriangle className="w-3.5 h-3.5" />}
            <span className="font-medium">
              {serverStatus.running ? `Running on ${serverStatus.address}` : 'Local API server is stopped'}
            </span>
          </div>
        </div>
      )}

      <div className="space-y-3">
        <div>
          <label className="block text-xs text-text-muted mb-1">
            Bind address
          </label>
          <input
            type="text"
            value={serverAddr}
            onChange={(e) => setServerAddr(e.target.value)}
            disabled={serverStatus?.running}
            className="w-full input disabled:opacity-50"
            placeholder="127.0.0.1:47321"
          />
        </div>

        {serverStatus?.running && serverStatus.token && (
          <div>
            <label className="block text-xs text-text-muted mb-1">
              Auth token
            </label>
            <div className="flex items-center gap-2">
              <div className="flex-1 px-3 py-2 rounded bg-bg-tertiary border border-border text-text-secondary text-xs font-mono truncate">
                {serverStatus.token}
              </div>
              <button
                onClick={handleCopyToken}
                className="btn-secondary text-xs flex items-center gap-1.5"
                title="Copy token to clipboard"
              >
                {tokenCopied ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                {tokenCopied ? 'Copied' : 'Copy'}
              </button>
              <button
                onClick={handleTestAuth}
                disabled={authTesting}
                className="btn-secondary text-xs flex items-center gap-1.5"
              >
                {authTesting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Shield className="w-3.5 h-3.5" />}
                {authTesting ? 'Testing...' : 'Test'}
              </button>
            </div>
          </div>
        )}

        {serverStatus && serverStatus.inboxCount > 0 && (
          <div className="flex items-center gap-2 text-xs text-text-muted">
            <Inbox className="w-3.5 h-3.5 text-accent" />
            <span>{serverStatus.inboxCount} capture(s) in inbox</span>
          </div>
        )}

        {serverStatus && serverStatus.recentCaptures.length > 0 && (
          <div className="space-y-1">
            <div className="text-xs text-text-muted">Recent captures</div>
            {serverStatus.recentCaptures.map((cap: CaptureInfo) => (
              <div key={cap.path} className="flex items-center justify-between px-2 py-1 rounded bg-bg-tertiary text-xs">
                <span className="text-text-secondary truncate" title={cap.path}>{cap.title}</span>
                <span className="text-text-muted">{new Date(cap.createdAt).toLocaleString()}</span>
              </div>
            ))}
          </div>
        )}

        <div className="text-xs text-text-muted flex items-start gap-2">
          <Info className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>
            Start the server to let the browser extension and mobile app connect to this vault.
            Write endpoints require the auth token above.
          </span>
        </div>

        <button
          onClick={handleToggleServer}
          disabled={serverToggling}
          className={`btn-primary text-xs flex items-center gap-1.5 ${serverStatus?.running ? 'bg-error hover:bg-red-600' : ''}`}
        >
          {serverToggling ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Server className="w-3.5 h-3.5" />}
          {serverToggling ? 'Working...' : serverStatus?.running ? 'Stop Server' : 'Start Server'}
        </button>
      </div>
    </section>
  );

  const renderAITab = () => (
    <section className="bg-bg-secondary rounded-lg border border-border p-4">
      <h2 className="text-sm font-medium text-text-primary mb-3 flex items-center gap-2">
        <Sparkles className="w-4 h-4 text-accent" />
        AI Provider
        {aiEnabled ? (
          <CheckCircle className="w-3.5 h-3.5 text-success" title="AI enabled" />
        ) : (
          <AlertTriangle className="w-3.5 h-3.5 text-warning" title="AI not configured" />
        )}
      </h2>

      {aiStatus && (
        <div className={`mb-3 px-3 py-2 rounded-lg border text-xs ${
          aiStatus.enabled
            ? 'bg-success/10 border-success/20 text-success'
            : 'bg-warning/10 border-warning/20 text-warning'
        }`}>
          <div className="flex items-center gap-2">
            {aiStatus.enabled ? <CheckCircle className="w-3.5 h-3.5" /> : <AlertTriangle className="w-3.5 h-3.5" />}
            <span className="font-medium">
              {aiStatus.enabled ? `${aiStatus.provider} · ${aiStatus.model}` : 'AI not reachable'}
            </span>
          </div>
          {aiStatus.error && (
            <p className="mt-1 ml-5 text-text-muted">{aiStatus.error}</p>
          )}
        </div>
      )}

      <div className="space-y-3">
        <div>
          <label className="block text-xs text-text-muted mb-1">
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
          <label className="block text-xs text-text-muted mb-1">
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
          <label className="block text-xs text-text-muted mb-1">
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

        <div className="text-xs text-text-muted flex items-start gap-2">
          <Info className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>
            {providerHelp[provider].replace('{model}', model || DEFAULT_MODELS[provider])}
            {provider === 'ollama' && (
              <> See <a href="https://ollama.com" target="_blank" rel="noopener noreferrer" className="text-accent hover:underline">ollama.com</a>.</>
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
  );

  const renderAppearanceTab = () => {
    const options: { value: Theme; label: string; icon: React.ReactNode }[] = [
      { value: 'light', label: 'Light', icon: <Sun className="w-5 h-5" /> },
      { value: 'dark', label: 'Dark', icon: <Moon className="w-5 h-5" /> },
      { value: 'system', label: 'System', icon: <Monitor className="w-5 h-5" /> },
    ];

    return (
      <section className="bg-bg-secondary rounded-lg border border-border p-4">
        <h2 className="text-sm font-medium text-text-primary mb-3 flex items-center gap-2">
          {resolved === 'dark' ? <Moon className="w-4 h-4 text-accent" /> : <Sun className="w-4 h-4 text-accent" />}
          Appearance
        </h2>
        <p className="text-xs text-text-muted mb-3">
          Choose how AgentVault looks. System follows your OS setting.
        </p>
        <div className="grid grid-cols-3 gap-2">
          {options.map((option) => {
            const selected = theme === option.value;
            return (
              <button
                key={option.value}
                onClick={() => setTheme(option.value)}
                className={`flex flex-col items-center gap-2 px-3 py-3 rounded-lg border text-xs font-medium transition-colors ${
                  selected
                    ? 'bg-accent/10 border-accent text-accent'
                    : 'bg-bg-tertiary border-border text-text-secondary hover:bg-bg-hover hover:text-text-primary'
                }`}
              >
                {option.icon}
                {option.label}
              </button>
            );
          })}
        </div>
      </section>
    );
  };

  const renderShortcutsTab = () => (
    <section className="bg-bg-secondary rounded-lg border border-border p-4">
      <h2 className="text-sm font-medium text-text-primary mb-3">
        Keyboard Shortcuts
      </h2>
      <div className="space-y-1.5 text-sm">
        {shortcuts.map(({ key, desc }) => (
          <div key={key} className="flex justify-between items-center">
            <span className="text-text-muted text-xs">{desc}</span>
            <kbd className="px-1.5 py-0.5 rounded bg-bg-tertiary text-text-secondary text-[10px] font-mono">
              {key}
            </kbd>
          </div>
        ))}
      </div>
    </section>
  );

  const renderActiveTab = () => {
    switch (activeTab) {
      case 'vault': return renderVaultTab();
      case 'ai': return renderAITab();
      case 'server': return renderServerTab();
      case 'appearance': return renderAppearanceTab();
      case 'shortcuts': return renderShortcutsTab();
      default: return null;
    }
  };

  return (
    <div className="flex flex-col h-full bg-bg-primary relative">
      {/* Header */}
      <div className="px-6 py-4 border-b border-border bg-bg-secondary">
        <h1 className="text-lg font-semibold text-text-primary">Settings</h1>
      </div>

      {/* Tabs */}
      <div className="px-6 pt-4 border-b border-border bg-bg-primary">
        <nav className="flex gap-1 overflow-x-auto" aria-label="Settings tabs">
          {tabs.map((tab) => {
            const active = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex items-center gap-1.5 px-3 py-2 rounded-t-lg text-xs font-medium whitespace-nowrap transition-colors border-b-2 ${
                  active
                    ? 'text-accent border-accent bg-bg-secondary'
                    : 'text-text-muted border-transparent hover:text-text-primary hover:bg-bg-hover'
                }`}
              >
                {tab.icon}
                {tab.label}
              </button>
            );
          })}
        </nav>
      </div>

      {/* Settings Content */}
      <div className="flex-1 overflow-auto p-6">
        <div className="max-w-2xl">
          {renderActiveTab()}
        </div>
      </div>

      {/* Error banner */}
      {error && (
        <div className="absolute top-4 right-4 max-w-sm px-4 py-3 rounded-lg bg-error/10 border border-error/20 text-sm text-error flex items-start gap-2.5 shadow-lg">
          <AlertTriangle className="w-4 h-4 mt-0.5 flex-shrink-0" />
          <span className="flex-1">{error}</span>
          <button onClick={() => setError('')} className="hover:opacity-70 flex-shrink-0">
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}

      {/* Toast */}
      {toast && (
        <div className="absolute bottom-4 left-1/2 -translate-x-1/2 px-4 py-2 bg-accent text-white text-sm rounded-lg shadow-lg flex items-center gap-2">
          {toast}
          <button onClick={() => setToast('')} className="hover:opacity-70">
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      )}
    </div>
  );
}
