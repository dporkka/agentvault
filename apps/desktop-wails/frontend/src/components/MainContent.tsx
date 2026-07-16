import { Suspense, lazy } from 'react';
import DashboardView from './DashboardView';
import SearchView from './SearchView';
import ProjectDashboard from './ProjectDashboard';
import DecisionDashboard from './DecisionDashboard';
import TagBrowser from './TagBrowser';
import SettingsView from './SettingsView';
import type { ViewName, VaultStatus } from '../types';
const EditorView = lazy(() => import('./EditorView'));

interface Props {
  activeView: ViewName;
  onViewChange: (view: ViewName) => void;
  selectedNotePath: string | null;
  onOpenNote: (path: string) => void;
  aiPanelOpen: boolean;
  onToggleAIPanel: () => void;
  vaultPath: string;
  vaultStatus: VaultStatus;
}

export default function MainContent({
  activeView,
  onViewChange,
  selectedNotePath,
  onOpenNote,
  aiPanelOpen,
  onToggleAIPanel,
  vaultPath,
  vaultStatus,
}: Props) {
  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {activeView === 'dashboard' && (
        <DashboardView
          vaultStatus={vaultStatus}
          onOpenNote={onOpenNote}
        />
      )}
      {activeView === 'editor' && (
        <Suspense
          fallback={
            <div className="flex-1 flex items-center justify-center text-text-muted text-sm">
              Loading editor…
            </div>
          }
        >
          <EditorView
            notePath={selectedNotePath}
            vaultPath={vaultPath}
            aiPanelOpen={aiPanelOpen}
            onToggleAIPanel={onToggleAIPanel}
          />
        </Suspense>
      )}
      {activeView === 'search' && (
        <SearchView onOpenNote={onOpenNote} />
      )}
      {activeView === 'projects' && (
        <ProjectDashboard onOpenNote={onOpenNote} />
      )}
      {activeView === 'decisions' && (
        <DecisionDashboard onOpenNote={onOpenNote} />
      )}
      {activeView === 'tags' && (
        <TagBrowser onOpenNote={onOpenNote} onSearchTag={() => onViewChange('search')} />
      )}
      {activeView === 'settings' && (
        <SettingsView vaultPath={vaultPath} />
      )}
    </div>
  );
}
