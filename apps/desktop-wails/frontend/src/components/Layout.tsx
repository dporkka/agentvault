import { useState, useCallback, useEffect } from 'react';
import Sidebar from './Sidebar';
import MainContent from './MainContent';
import AIPanel from './AIPanel';
import { Loader2, CheckCircle, AlertTriangle, Server, Inbox, Shield } from './Icons';
import type { VaultStatus, ViewName, IndexingStatus, AIStatus, ServerStatus } from '../types';

interface Props {
  vaultStatus: VaultStatus;
  onVaultChanged: () => void;
}

export default function Layout({ vaultStatus, onVaultChanged }: Props) {
  const [activeView, setActiveView] = useState<ViewName>('search');
  const [aiPanelOpen, setAiPanelOpen] = useState(false);
  const [selectedNotePath, setSelectedNotePath] = useState<string | null>(null);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [indexStatus, setIndexStatus] = useState<IndexingStatus>({ isIndexing: false, noteCount: vaultStatus.noteCount });
  const [aiStatus, setAiStatus] = useState<AIStatus | null>(null);
  const [serverStatus, setServerStatus] = useState<ServerStatus | null>(null);

  const handleOpenNote = useCallback((path: string) => {
    setSelectedNotePath(path);
    setActiveView('editor');
  }, []);

  const handleNewNote = useCallback(() => {
    setSelectedNotePath(null);
    setActiveView('editor');
  }, []);

  useEffect(() => {
    const refresh = async () => {
      try {
        const status = await window.go.main.IndexService.GetStatus();
        setIndexStatus(status);
      } catch (err) {
        console.error('Failed to load index status:', err);
      }
      try {
        const status = await window.go.main.AIService.GetStatus();
        setAiStatus(status);
      } catch (err) {
        console.error('Failed to load AI status:', err);
      }
      try {
        const status = await window.go.main.ServerService.GetServerStatus();
        setServerStatus(status);
      } catch (err) {
        console.error('Failed to load server status:', err);
      }
    };
    refresh();
    const id = setInterval(refresh, 3000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    setIndexStatus(prev => ({ ...prev, noteCount: vaultStatus.noteCount }));
  }, [vaultStatus.noteCount]);

  const vaultName = vaultStatus.path.split('/').pop() || 'AgentVault';

  return (
    <div className="flex h-full w-full bg-[var(--bg-primary)]">
      {/* Sidebar */}
      <Sidebar
        vaultStatus={vaultStatus}
        activeView={activeView}
        onViewChange={setActiveView}
        onOpenNote={handleOpenNote}
        onNewNote={handleNewNote}
        onVaultChanged={onVaultChanged}
        collapsed={sidebarCollapsed}
        onToggleCollapse={() => setSidebarCollapsed(!sidebarCollapsed)}
      />

      {/* Main Content */}
      <div className="flex-1 flex flex-col min-w-0">
        <MainContent
          activeView={activeView}
          onViewChange={setActiveView}
          selectedNotePath={selectedNotePath}
          onOpenNote={handleOpenNote}
          aiPanelOpen={aiPanelOpen}
          onToggleAIPanel={() => setAiPanelOpen(!aiPanelOpen)}
          vaultPath={vaultStatus.path}
        />

        {/* Global status bar */}
        <div className="h-7 px-3 flex items-center justify-between border-t border-[var(--border)] bg-[var(--bg-secondary)] text-[11px] text-[var(--text-muted)] select-none">
          <div className="flex items-center gap-3 min-w-0">
            <span className="truncate" title={vaultStatus.path}>
              {vaultName}
            </span>
            <span className="text-[var(--border)]">|</span>
            <span>{indexStatus.noteCount} notes</span>
          </div>
          <div className="flex items-center gap-3">
            {serverStatus?.running ? (
              <span className="flex items-center gap-1 text-[var(--success)]" title={`Local API running on ${serverStatus.address}`}>
                <Server className="w-3 h-3" />
                API {serverStatus.address.replace(/^127\.0\.0\.1:/, ':')}
              </span>
            ) : (
              <span className="flex items-center gap-1 text-[var(--text-muted)]" title="Local API not running">
                <Server className="w-3 h-3" />
                API off
              </span>
            )}
            {serverStatus?.running && (
              <>
                <span className="flex items-center gap-1 text-[var(--success)]" title="Auth token configured">
                  <Shield className="w-3 h-3" />
                  Auth
                </span>
                {serverStatus.inboxCount > 0 && (
                  <span className="flex items-center gap-1 text-[var(--accent)]" title={`${serverStatus.inboxCount} capture(s) in inbox`}>
                    <Inbox className="w-3 h-3" />
                    {serverStatus.inboxCount}
                  </span>
                )}
              </>
            )}
            {indexStatus.isIndexing ? (
              <span className="flex items-center gap-1 text-[var(--accent)]" title="Indexing vault">
                <Loader2 className="w-3 h-3 animate-spin" />
                Indexing...
              </span>
            ) : (
              <span className="flex items-center gap-1 text-[var(--success)]" title="Indexing up to date">
                <CheckCircle className="w-3 h-3" />
                Indexed
              </span>
            )}
            {aiStatus?.enabled ? (
              <span className="flex items-center gap-1 text-[var(--success)]" title={`${aiStatus.provider} · ${aiStatus.model}`}>
                <CheckCircle className="w-3 h-3" />
                AI ready
              </span>
            ) : (
              <span className="flex items-center gap-1 text-yellow-400" title={aiStatus?.error || 'AI not configured'}>
                <AlertTriangle className="w-3 h-3" />
                AI offline
              </span>
            )}
          </div>
        </div>
      </div>

      {/* AI Panel */}
      {aiPanelOpen && (
        <AIPanel
          onClose={() => setAiPanelOpen(false)}
          onOpenNote={handleOpenNote}
          vaultPath={vaultStatus.path}
        />
      )}
    </div>
  );
}
