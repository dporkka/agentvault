import { useState, useCallback, useEffect } from 'react';
import Sidebar from './Sidebar';
import MainContent from './MainContent';
import AIPanel from './AIPanel';
import { Loader2, CheckCircle, AlertTriangle, Server, Inbox, Shield } from './Icons';
import { useTheme } from '../hooks/useTheme';
import type { VaultStatus, ViewName, IndexingStatus, AIStatus, ServerStatus } from '../types';

interface Props {
  vaultStatus: VaultStatus;
  onVaultChanged: () => void;
}

function SunIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
  );
}

function MoonIcon({ className }: { className?: string }) {
  return (
    <svg className={className} fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
    </svg>
  );
}

function StatusIcon({
  icon,
  title,
  className,
}: {
  icon: React.ReactNode;
  title: string;
  className?: string;
}) {
  return (
    <button
      type="button"
      title={title}
      className={`p-1.5 rounded-md hover:bg-bg-hover transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-accent ${className || ''}`}
    >
      {icon}
    </button>
  );
}

export default function Layout({ vaultStatus, onVaultChanged }: Props) {
  const [activeView, setActiveView] = useState<ViewName>('dashboard');
  const [aiPanelOpen, setAiPanelOpen] = useState(false);
  const [selectedNotePath, setSelectedNotePath] = useState<string | null>(null);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [indexStatus, setIndexStatus] = useState<IndexingStatus>({ isIndexing: false, noteCount: vaultStatus.noteCount });
  const [aiStatus, setAiStatus] = useState<AIStatus | null>(null);
  const [serverStatus, setServerStatus] = useState<ServerStatus | null>(null);
  const { theme, setTheme, resolved } = useTheme();

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

  const cycleTheme = useCallback(() => {
    const next: Record<ReturnType<typeof useTheme>['theme'], ReturnType<typeof useTheme>['theme']> = {
      light: 'dark',
      dark: 'system',
      system: 'light',
    };
    setTheme(next[theme]);
  }, [theme, setTheme]);

  return (
    <div className="flex h-full w-full bg-bg-primary">
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
          vaultStatus={vaultStatus}
        />

        {/* Global status bar */}
        <div className="h-10 px-4 flex items-center gap-4 border-t border-border bg-bg-secondary text-xs text-text-muted select-none">
          {/* Left group: vault name + note count */}
          <div className="flex items-center gap-3 min-w-0 shrink-0">
            <span className="truncate font-medium text-text-primary" title={vaultStatus.path}>
              {vaultName}
            </span>
            <span className="text-border">|</span>
            <span>{indexStatus.noteCount} notes</span>
          </div>

          {/* Center group: flexible spacer (reserved for future status messages) */}
          <div className="flex-1 min-w-0" />

          {/* Right group: API, auth, inbox, indexing, AI, theme */}
          <div className="flex items-center gap-1 shrink-0">
            {serverStatus?.running ? (
              <StatusIcon
                className="text-success"
                title={`Local API running on ${serverStatus.address}`}
                icon={<Server className="w-4 h-4" />}
              />
            ) : (
              <StatusIcon
                title="Local API not running"
                icon={<Server className="w-4 h-4" />}
              />
            )}
            {serverStatus?.running && (
              <>
                <StatusIcon
                  className="text-success"
                  title="Auth token configured"
                  icon={<Shield className="w-4 h-4" />}
                />
                {serverStatus.inboxCount > 0 && (
                  <StatusIcon
                    className="text-accent"
                    title={`${serverStatus.inboxCount} capture(s) in inbox`}
                    icon={<Inbox className="w-4 h-4" />}
                  />
                )}
              </>
            )}
            {indexStatus.isIndexing ? (
              <StatusIcon
                className="text-accent"
                title="Indexing vault"
                icon={<Loader2 className="w-4 h-4 animate-spin" />}
              />
            ) : (
              <StatusIcon
                className="text-success"
                title="Indexing up to date"
                icon={<CheckCircle className="w-4 h-4" />}
              />
            )}
            {aiStatus?.enabled ? (
              <StatusIcon
                className="text-success"
                title={`${aiStatus.provider} · ${aiStatus.model}`}
                icon={<CheckCircle className="w-4 h-4" />}
              />
            ) : (
              <StatusIcon
                className="text-warning"
                title={aiStatus?.error || 'AI not configured'}
                icon={<AlertTriangle className="w-4 h-4" />}
              />
            )}

            <div className="w-px h-4 bg-border mx-1" />

            <button
              type="button"
              onClick={cycleTheme}
              title={`Theme: ${theme} (${resolved})`}
              className="p-1.5 rounded-md hover:bg-bg-hover transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-accent"
            >
              {resolved === 'dark' ? (
                <MoonIcon className="w-4 h-4" />
              ) : (
                <SunIcon className="w-4 h-4" />
              )}
            </button>
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
