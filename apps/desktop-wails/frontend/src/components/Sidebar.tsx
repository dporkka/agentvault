import { useState, useEffect, useCallback } from 'react';
import {
  SearchIcon,
  FileText,
  FolderTree,
  Sparkles,
  SettingsIcon,
  LayoutDashboard,
  GitBranch,
  PanelRight,
  ChevronRight,
  ChevronDown,
  Plus,
  Activity,
  Loader2,
  CheckCircle,
  AlertTriangle,
} from './Icons';
import type { VaultStatus, ViewName, IndexingStatus, AIStatus } from '../types';

interface Props {
  vaultStatus: VaultStatus;
  activeView: ViewName;
  onViewChange: (view: ViewName) => void;
  onOpenNote: (path: string) => void;
  onNewNote: () => void;
  onVaultChanged: () => void;
  collapsed: boolean;
  onToggleCollapse: () => void;
}

interface TreeItem {
  name: string;
  path: string;
  type: 'folder' | 'file';
  children?: TreeItem[];
}

const navItems: { id: ViewName; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: 'search', label: 'Search', icon: SearchIcon },
  { id: 'editor', label: 'Editor', icon: FileText },
  { id: 'projects', label: 'Projects', icon: LayoutDashboard },
  { id: 'decisions', label: 'Decisions', icon: FolderTree },
  { id: 'settings', label: 'Settings', icon: SettingsIcon },
];

export default function Sidebar({
  vaultStatus,
  activeView,
  onViewChange,
  onOpenNote,
  onNewNote,
  collapsed,
  onToggleCollapse,
}: Props) {
  const [vaultTree, setVaultTree] = useState<TreeItem[]>([]);
  const [expandedFolders, setExpandedFolders] = useState<Set<string>>(new Set(['10-notes', '20-projects', '30-decisions', '40-research']));
  const [recentNotes, setRecentNotes] = useState<Array<{ title: string; path: string }>>([]);
  const [indexStatus, setIndexStatus] = useState<IndexingStatus>({ isIndexing: false, noteCount: vaultStatus.noteCount });
  const [aiStatus, setAiStatus] = useState<AIStatus | null>(null);

  const loadTree = useCallback(async () => {
    try {
      const tree = await window.go.main.NoteService.GetRecent(5);
      setRecentNotes(tree.map(n => ({ title: n.title, path: n.path })));

      // Build a simple folder tree from the vault
      // In a real app, this would scan the filesystem
      setVaultTree([
        { name: '00-inbox', path: '00-inbox', type: 'folder', children: [] },
        { name: '10-notes', path: '10-notes', type: 'folder', children: [] },
        { name: '20-projects', path: '20-projects', type: 'folder', children: [] },
        { name: '30-decisions', path: '30-decisions', type: 'folder', children: [] },
        { name: '40-research', path: '40-research', type: 'folder', children: [] },
      ]);
    } catch (err) {
      console.error('Failed to load tree:', err);
    }
  }, []);

  const refreshStatus = useCallback(async () => {
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
      setAiStatus(null);
    }
  }, []);

  useEffect(() => {
    loadTree();
    refreshStatus();
    const id = setInterval(refreshStatus, 3000);
    return () => clearInterval(id);
  }, [loadTree, refreshStatus]);

  useEffect(() => {
    setIndexStatus(prev => ({ ...prev, noteCount: vaultStatus.noteCount }));
  }, [vaultStatus.noteCount]);

  const toggleFolder = (name: string) => {
    setExpandedFolders(prev => {
      const next = new Set(prev);
      if (next.has(name)) {
        next.delete(name);
      } else {
        next.add(name);
      }
      return next;
    });
  };

  const vaultName = vaultStatus.path.split('/').pop() || 'AgentVault';

  if (collapsed) {
    return (
      <div className="w-12 flex flex-col items-center py-3 gap-1 bg-[var(--bg-secondary)] border-r border-[var(--border)]">
        {navItems.map(item => (
          <button
            key={item.id}
            onClick={() => onViewChange(item.id)}
            className={`p-2 rounded-lg transition-colors ${
              activeView === item.id
                ? 'text-[var(--accent)] bg-[var(--accent)]/10'
                : 'text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'
            }`}
            title={item.label}
          >
            <item.icon className="w-5 h-5" />
          </button>
        ))}
      </div>
    );
  }

  return (
    <div className="w-60 flex flex-col bg-[var(--bg-secondary)] border-r border-[var(--border)]">
      {/* Vault Header */}
      <div className="px-3 py-2.5 border-b border-[var(--border)]">
        <div className="flex items-center gap-2">
          <div className="w-7 h-7 rounded-lg bg-[var(--accent)]/10 flex items-center justify-center">
            <Sparkles className="w-4 h-4 text-[var(--accent)]" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="text-sm font-medium text-[var(--text-primary)] truncate" title={vaultName}>
              {vaultName}
            </div>
            <div className="text-xs text-[var(--text-muted)]">
              {indexStatus.noteCount} notes
            </div>
          </div>
        </div>
      </div>

      {/* New Note Button */}
      <div className="px-3 py-2">
        <button
          onClick={onNewNote}
          className="w-full flex items-center justify-center gap-2 btn-primary"
        >
          <Plus className="w-4 h-4" />
          New Note
        </button>
      </div>

      {/* Navigation */}
      <div className="px-2 py-1">
        {navItems.map(item => (
          <button
            key={item.id}
            onClick={() => onViewChange(item.id)}
            className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-sm transition-colors ${
              activeView === item.id
                ? 'text-[var(--accent)] bg-[var(--accent)]/10 font-medium'
                : 'text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)]'
            }`}
          >
            <item.icon className="w-4 h-4" />
            {item.label}
          </button>
        ))}
      </div>

      {/* Divider */}
      <div className="mx-3 my-2 border-t border-[var(--border)]" />

      {/* Folder Tree */}
      <div className="flex-1 overflow-auto px-2">
        <div className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wider px-2.5 py-1.5">
          Folders
        </div>
        {vaultTree.map(item => (
          <div key={item.path}>
            <button
              onClick={() => toggleFolder(item.name)}
              className="w-full flex items-center gap-1 px-2 py-1 text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] rounded"
            >
              {expandedFolders.has(item.name) ? (
                <ChevronDown className="w-3 h-3" />
              ) : (
                <ChevronRight className="w-3 h-3" />
              )}
              <FolderTree className="w-3.5 h-3.5" />
              {item.name}
            </button>
          </div>
        ))}
      </div>

      {/* Recent */}
      {recentNotes.length > 0 && (
        <>
          <div className="mx-3 my-2 border-t border-[var(--border)]" />
          <div className="px-2 pb-2 max-h-40 overflow-auto">
            <div className="text-xs font-medium text-[var(--text-muted)] uppercase tracking-wider px-2.5 py-1.5">
              Recent
            </div>
            {recentNotes.map(note => (
              <button
                key={note.path}
                onClick={() => onOpenNote(note.path)}
                className="w-full flex items-center gap-2 px-2.5 py-1 text-sm text-[var(--text-secondary)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-hover)] rounded truncate"
                title={note.title}
              >
                <FileText className="w-3.5 h-3.5 flex-shrink-0" />
                <span className="truncate">{note.title}</span>
              </button>
            ))}
          </div>
        </>
      )}

      {/* Footer */}
      <div className="px-3 py-2 border-t border-[var(--border)] space-y-1.5">
        {/* Status indicators */}
        <div className="space-y-1">
          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]" title="Indexing status">
            {indexStatus.isIndexing ? (
              <>
                <Loader2 className="w-3.5 h-3.5 text-[var(--accent)] animate-spin" />
                <span className="text-[var(--accent)]">Indexing...</span>
              </>
            ) : (
              <>
                <Activity className="w-3.5 h-3.5 text-[var(--success)]" />
                <span>Indexed {indexStatus.noteCount} notes</span>
              </>
            )}
          </div>

          <div className="flex items-center gap-2 text-xs text-[var(--text-muted)]" title={aiStatus?.error || (aiStatus?.enabled ? `${aiStatus.provider} · ${aiStatus.model}` : 'AI not configured')}>
            {aiStatus?.enabled ? (
              <>
                <CheckCircle className="w-3.5 h-3.5 text-[var(--success)]" />
                <span className="truncate">
                  AI: {aiStatus.provider}
                </span>
              </>
            ) : (
              <>
                <AlertTriangle className="w-3.5 h-3.5 text-yellow-400" />
                <span className="truncate">
                  {aiStatus?.error ? `AI: ${aiStatus.error}` : 'AI not configured'}
                </span>
              </>
            )}
          </div>
        </div>

        <div className="flex items-center justify-between pt-1.5 border-t border-[var(--border)]">
          <div className="flex items-center gap-1.5 text-xs text-[var(--text-muted)]">
            <GitBranch className="w-3.5 h-3.5" />
            <span>main</span>
          </div>
          <button
            onClick={onToggleCollapse}
            className="p-1 rounded hover:bg-[var(--bg-hover)] text-[var(--text-muted)]"
            title="Collapse sidebar"
          >
            <PanelRight className="w-4 h-4" />
          </button>
        </div>
      </div>
    </div>
  );
}
