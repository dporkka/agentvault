import { Suspense, lazy } from 'react';
import SearchView from './SearchView';
import ProjectDashboard from './ProjectDashboard';
import DecisionDashboard from './DecisionDashboard';
import SettingsView from './SettingsView';
import type { ViewName } from '../types';

const EditorView = lazy(() => import('./EditorView'));

interface Props {
  activeView: ViewName;
  onViewChange: (view: ViewName) => void;
  selectedNotePath: string | null;
  onOpenNote: (path: string) => void;
  aiPanelOpen: boolean;
  onToggleAIPanel: () => void;
  vaultPath: string;
}

export default function MainContent({
  activeView,
  selectedNotePath,
  onOpenNote,
  aiPanelOpen,
  onToggleAIPanel,
  vaultPath,
}: Props) {
  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {activeView === 'editor' && (
        <Suspense
          fallback={
            <div className="flex-1 flex items-center justify-center text-[var(--text-muted)] text-sm">
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
      {activeView === 'settings' && (
        <SettingsView vaultPath={vaultPath} />
      )}
    </div>
  );
}
