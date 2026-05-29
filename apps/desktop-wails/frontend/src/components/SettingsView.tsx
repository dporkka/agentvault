import { useState, useEffect, useCallback } from 'react';
import { HardDrive, Sparkles, CheckCircle, AlertTriangle, X } from './Icons';

interface Props {
  vaultPath: string;
}

export default function SettingsView({ vaultPath }: Props) {
  const [aiEnabled, setAiEnabled] = useState(false);
  const [indexStatus, setIndexStatus] = useState({ isIndexing: false, noteCount: 0 });
  const [reindexing, setReindexing] = useState(false);
  const [ollamaUrl, setOllamaUrl] = useState('http://localhost:11434');
  const [model, setModel] = useState('llama3.1');
  const [toast, setToast] = useState('');

  useEffect(() => {
    window.go.main.AIService.IsAIEnabled().then(setAiEnabled).catch(() => {});
    window.go.main.IndexService.GetStatus().then(setIndexStatus).catch(() => {});
  }, []);

  const handleReindex = useCallback(async () => {
    setReindexing(true);
    try {
      await window.go.main.IndexService.Index(true);
      const status = await window.go.main.IndexService.GetStatus();
      setIndexStatus(status);
      setToast('Vault reindexed successfully');
      setTimeout(() => setToast(''), 3000);
    } catch (err: any) {
      setToast(`Error: ${err.message}`);
    } finally {
      setReindexing(false);
    }
  }, []);

  const handleSaveAIConfig = useCallback(async () => {
    // In a real app, this would save to the vault config
    setToast('AI settings saved (restart required)');
    setTimeout(() => setToast(''), 3000);
  }, [ollamaUrl, model]);

  return (
    <div className="flex flex-col h-full bg-[var(--bg-primary)]">
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
              <div className="flex justify-between">
                <span className="text-[var(--text-muted)]">Path</span>
                <span className="text-[var(--text-primary)] font-mono text-xs">{vaultPath}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-[var(--text-muted)]">Notes indexed</span>
                <span className="text-[var(--text-primary)]">{indexStatus.noteCount}</span>
              </div>
              <div className="flex justify-between">
                <span className="text-[var(--text-muted)]">Version</span>
                <span className="text-[var(--text-primary)]">0.1.0</span>
              </div>
            </div>
            <button
              onClick={handleReindex}
              disabled={reindexing}
              className="mt-3 btn-secondary text-xs"
            >
              {reindexing ? 'Reindexing...' : 'Force Reindex'}
            </button>
          </section>

          {/* AI Settings */}
          <section className="bg-[var(--bg-secondary)] rounded-lg border border-[var(--border)] p-4">
            <h2 className="text-sm font-medium text-[var(--text-primary)] mb-3 flex items-center gap-2">
              <Sparkles className="w-4 h-4 text-[var(--accent)]" />
              AI Provider
              {aiEnabled ? (
                <CheckCircle className="w-3.5 h-3.5 text-green-400" />
              ) : (
                <AlertTriangle className="w-3.5 h-3.5 text-yellow-400" />
              )}
            </h2>

            <div className="space-y-3">
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">
                  Ollama URL
                </label>
                <input
                  type="text"
                  value={ollamaUrl}
                  onChange={(e) => setOllamaUrl(e.target.value)}
                  className="w-full input"
                  placeholder="http://localhost:11434"
                />
              </div>
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-1">
                  Model
                </label>
                <input
                  type="text"
                  value={model}
                  onChange={(e) => setModel(e.target.value)}
                  className="w-full input"
                  placeholder="llama3.1"
                />
              </div>
              <div className="text-xs text-[var(--text-muted)]">
                Install Ollama from <a href="https://ollama.com" target="_blank" rel="noopener noreferrer" className="text-[var(--accent)] hover:underline">ollama.com</a>, then run:
                <code className="block mt-1 px-2 py-1 bg-[var(--bg-tertiary)] rounded font-mono text-[var(--text-secondary)]">
                  ollama pull {model}
                </code>
              </div>
              <button
                onClick={handleSaveAIConfig}
                className="btn-primary text-xs"
              >
                Save AI Settings
              </button>
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
