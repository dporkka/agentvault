import { useState, useCallback } from 'react';
import Sidebar from './Sidebar';
import MainContent from './MainContent';
import AIPanel from './AIPanel';
import type { VaultStatus, ViewName } from '../types';

interface Props {
  vaultStatus: VaultStatus;
  onVaultChanged: () => void;
}

export default function Layout({ vaultStatus, onVaultChanged }: Props) {
  const [activeView, setActiveView] = useState<ViewName>('search');
  const [aiPanelOpen, setAiPanelOpen] = useState(false);
  const [selectedNotePath, setSelectedNotePath] = useState<string | null>(null);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);

  const handleOpenNote = useCallback((path: string) => {
    setSelectedNotePath(path);
    setActiveView('editor');
  }, []);

  const handleNewNote = useCallback(() => {
    setSelectedNotePath(null);
    setActiveView('editor');
  }, []);

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
      <MainContent
        activeView={activeView}
        onViewChange={setActiveView}
        selectedNotePath={selectedNotePath}
        onOpenNote={handleOpenNote}
        aiPanelOpen={aiPanelOpen}
        onToggleAIPanel={() => setAiPanelOpen(!aiPanelOpen)}
        vaultPath={vaultStatus.path}
      />

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
