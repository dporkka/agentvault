import EditorView from './EditorView';
import SearchView from './SearchView';
import ProjectDashboard from './ProjectDashboard';
import DecisionDashboard from './DecisionDashboard';
import SettingsView from './SettingsView';
import type { ViewName } from '../types';

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
        <EditorView
          notePath={selectedNotePath}
          vaultPath={vaultPath}
          aiPanelOpen={aiPanelOpen}
          onToggleAIPanel={onToggleAIPanel}
        />
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
