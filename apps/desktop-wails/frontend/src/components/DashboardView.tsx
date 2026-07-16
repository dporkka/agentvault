import { useState, useEffect, useCallback } from 'react';
import {
  HardDrive,
  FileText,
  Activity,
  AlertTriangle,
  Sparkles,
  RefreshCw,
} from './Icons';
import type { VaultStatus, SearchResult, IndexingStatus, AIStatus } from '../types';

interface Props {
  vaultStatus: VaultStatus;
  onOpenNote: (path: string) => void;
}

interface DashboardData {
  recentNotes: SearchResult[];
  staleNotes: SearchResult[];
  indexStatus: IndexingStatus;
  aiStatus: AIStatus | null;
  aiEnabled: boolean;
  typeCounts: Record<string, number>;
}

const NOTE_TYPES = ['note', 'decision', 'task', 'meeting', 'source'] as const;
const TYPE_COLORS: Record<string, string> = {
  note: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  decision: 'bg-purple-500/10 text-purple-400 border-purple-500/20',
  task: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  meeting: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
  source: 'bg-slate-500/10 text-slate-400 border-slate-500/20',
};

export default function DashboardView({ vaultStatus, onOpenNote }: Props) {
  const [data, setData] = useState<DashboardData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadDashboard = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [recentNotes, indexStatus, aiEnabled, noteResults] = await Promise.all([
        window.go.main.NoteService.GetRecent(50),
        window.go.main.IndexService.GetStatus(),
        window.go.main.AIService.IsAIEnabled(),
        window.go.main.NoteService.Search('', '', '', false, 0, 100),
      ]);

      let aiStatus: AIStatus | null = null;
      if (aiEnabled) {
        try {
          aiStatus = await window.go.main.AIService.GetStatus();
        } catch {
          // AI status unavailable
        }
      }

      // Compute type counts from search results
      const typeCounts: Record<string, number> = {};
      for (const t of NOTE_TYPES) {
        typeCounts[t] = noteResults.filter((r) => r.type === t).length;
      }

      // Sort recent by updatedAt descending, take top 5
      const sorted = [...recentNotes].sort(
        (a, b) => new Date(b.updatedAt || '').getTime() - new Date(a.updatedAt || '').getTime(),
      );
      const recent5 = sorted.slice(0, 5);

      // Stale notes: take the oldest from sorted, bottom 5
      const stale5 = sorted.slice(-5).reverse();

      setData({ recentNotes: recent5, staleNotes: stale5, indexStatus, aiStatus, aiEnabled, typeCounts });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load dashboard');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadDashboard();
  }, [loadDashboard]);

  const handleReindex = async () => {
    try {
      await window.go.main.IndexService.Index(true);
      loadDashboard();
    } catch {
      // silently fail
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-sm text-text-muted">Loading dashboard...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3">
        <AlertTriangle className="w-8 h-8 text-red-400" />
        <p className="text-sm text-red-400">{error}</p>
        <button
          onClick={loadDashboard}
          className="px-3 py-1.5 text-xs text-text-secondary bg-bg-tertiary rounded hover:bg-bg-hover transition-colors"
        >
          Retry
        </button>
      </div>
    );
  }

  if (!data) return null;

  const vaultName = vaultStatus.path.split('/').pop() || 'AgentVault';

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {/* Header */}
      <div className="border-b border-border px-6 py-4">
        <h1 className="text-lg font-semibold text-text-primary">Dashboard</h1>
        <p className="text-sm text-text-muted mt-1">{vaultName}</p>
      </div>

      <div className="flex-1 overflow-y-auto px-6 py-4 space-y-6">
        {/* Stats row */}
        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3">
          <StatCard icon={<FileText className="w-4 h-4" />} label="Notes" value={vaultStatus.noteCount} />
          <StatCard icon={<Activity className="w-4 h-4" />} label="Indexed" value={data.indexStatus.noteCount} />
          <StatCard
            icon={<Sparkles className="w-4 h-4" />}
            label="AI"
            value={data.aiEnabled ? (data.aiStatus?.provider || 'On') : 'Off'}
          />
          <StatCard
            icon={<HardDrive className="w-4 h-4" />}
            label="Vault"
            value={vaultName}
            mono
          />
        </div>

        {/* Type breakdown */}
        <Section title="Note Types">
          <div className="flex flex-wrap gap-2">
            {NOTE_TYPES.map((t) => (
              <span
                key={t}
                className={`px-3 py-1 text-xs font-medium rounded-full border capitalize ${TYPE_COLORS[t] || 'bg-bg-tertiary text-text-secondary border-border'}`}
              >
                {t} ({data.typeCounts[t] || 0})
              </span>
            ))}
          </div>
        </Section>

        {/* Recent notes */}
        <Section title="Recent Notes">
          <NoteList notes={data.recentNotes} onOpenNote={onOpenNote} />
        </Section>

        {/* Stale notes */}
        {data.staleNotes.length > 0 && (
          <Section title="Stale Notes" subtitle="Oldest modified">
            <NoteList notes={data.staleNotes} onOpenNote={onOpenNote} />
          </Section>
        )}

        {/* Index health */}
        <Section title="Index Health">
          <div className="space-y-2 text-sm">
            <InfoRow label="Status" value={data.indexStatus.isIndexing ? 'Indexing...' : 'Idle'} />
            <InfoRow label="Notes indexed" value={String(data.indexStatus.noteCount)} />
            <button
              onClick={handleReindex}
              className="flex items-center gap-2 px-3 py-1.5 text-xs text-accent bg-accent/10 rounded-md hover:bg-accent/20 transition-colors"
            >
              <RefreshCw className="w-3 h-3" />
              Re-index vault
            </button>
          </div>
        </Section>

        {/* AI Status */}
        <Section title="AI Provider">
          {data.aiEnabled && data.aiStatus ? (
            <div className="space-y-2 text-sm">
              <InfoRow label="Provider" value={data.aiStatus.provider} />
              <InfoRow label="Model" value={data.aiStatus.model} />
              {data.aiStatus.error && (
                <InfoRow label="Status" value={data.aiStatus.error} error />
              )}
            </div>
          ) : (
            <p className="text-sm text-text-muted">AI is not configured. Enable it in Settings.</p>
          )}
        </Section>
      </div>
    </div>
  );
}

/* ---- Sub-components ---- */

function StatCard({
  icon,
  label,
  value,
  mono,
}: {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  mono?: boolean;
}) {
  return (
    <div className="bg-bg-secondary border border-border rounded-lg p-3">
      <div className="flex items-center gap-2 text-text-muted mb-1">
        {icon}
        <span className="text-xs uppercase tracking-wide">{label}</span>
      </div>
      <p className={`text-lg font-semibold text-text-primary truncate ${mono ? 'font-mono text-sm' : ''}`}>
        {value}
      </p>
    </div>
  );
}

function Section({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <div className="flex items-baseline gap-2 mb-3">
        <h2 className="text-sm font-semibold text-text-primary uppercase tracking-wide">{title}</h2>
        {subtitle && <span className="text-xs text-text-muted">{subtitle}</span>}
      </div>
      {children}
    </div>
  );
}

function NoteList({
  notes,
  onOpenNote,
}: {
  notes: SearchResult[];
  onOpenNote: (path: string) => void;
}) {
  if (notes.length === 0) {
    return <p className="text-sm text-text-muted">No notes yet.</p>;
  }

  return (
    <div className="space-y-1">
      {notes.map((note) => (
        <button
          key={note.id}
          onClick={() => onOpenNote(note.path)}
          className="w-full text-left flex items-center gap-3 px-3 py-2 rounded-md hover:bg-bg-hover transition-colors group"
        >
          <span
            className={`px-2 py-0.5 text-[10px] font-medium rounded-full border capitalize ${TYPE_COLORS[note.type] || 'bg-bg-tertiary text-text-secondary border-border'}`}
          >
            {note.type}
          </span>
          <span className="text-sm text-text-primary group-hover:text-accent transition-colors truncate flex-1">
            {note.title}
          </span>
          {note.updatedAt && (
            <span className="text-xs text-text-muted flex-shrink-0">
              {formatRelative(note.updatedAt)}
            </span>
          )}
        </button>
      ))}
    </div>
  );
}

function InfoRow({
  label,
  value,
  error: isError,
}: {
  label: string;
  value: string;
  error?: boolean;
}) {
  return (
    <div className="flex justify-between items-center">
      <span className="text-text-muted">{label}</span>
      <span className={isError ? 'text-red-400' : 'text-text-primary'}>{value}</span>
    </div>
  );
}

function formatRelative(dateStr: string): string {
  const date = new Date(dateStr);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMin = Math.floor(diffMs / 60000);
  if (diffMin < 1) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHr = Math.floor(diffMin / 60);
  if (diffHr < 24) return `${diffHr}h ago`;
  const diffDay = Math.floor(diffHr / 24);
  if (diffDay < 30) return `${diffDay}d ago`;
  return date.toLocaleDateString();
}
