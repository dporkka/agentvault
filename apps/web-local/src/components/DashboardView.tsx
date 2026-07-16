import React, { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { api } from '@/api/client';
import { typeBadgeClass } from '@/utils/styles';
import type { VaultStatus, SearchResult } from '@agentvault/contract';

const NOTE_TYPES = ['note', 'decision', 'task', 'meeting', 'source'] as const;

const DashboardView: React.FC = () => {
  const navigate = useNavigate();
  const [vaultStatus, setVaultStatus] = useState<VaultStatus | null>(null);
  const [recentNotes, setRecentNotes] = useState<SearchResult[]>([]);
  const [staleNotes, setStaleNotes] = useState<SearchResult[]>([]);
  const [typeCounts, setTypeCounts] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const loadDashboard = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [status, recent, stale] = await Promise.all([
        api.getVaultStatus(),
        api.getRecent({ limit: 50 }),
        api.getStale({ limit: 10 }),
      ]);

      setVaultStatus(status);
      setStaleNotes(stale);

      // Sort recent by updatedAt descending
      const sorted = [...recent].sort(
        (a, b) => new Date(b.updatedAt || '').getTime() - new Date(a.updatedAt || '').getTime(),
      );
      setRecentNotes(sorted.slice(0, 5));

      // Compute type counts from recent+stale union
      const counts: Record<string, number> = {};
      for (const t of NOTE_TYPES) {
        counts[t] = 0;
      }
      for (const note of [...recent, ...stale]) {
        if (counts[note.type] !== undefined) {
          counts[note.type]++;
        }
      }
      setTypeCounts(counts);
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
      await api.triggerIndex({ force: true });
      loadDashboard();
    } catch {
      // silently fail
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-sm text-vault-text-muted">Loading dashboard...</div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex flex-col items-center justify-center h-full gap-3">
        <svg className="w-8 h-8 text-vault-error" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 9v3.75m-9.303 3.376c-.866 1.5.217 3.374 1.948 3.374h14.71c1.73 0 2.813-1.874 1.948-3.374L13.949 3.378c-.866-1.5-3.032-1.5-3.898 0L2.697 16.126ZM12 15.75h.007v.008H12v-.008Z" />
        </svg>
        <p className="text-sm text-vault-error">{error}</p>
        <button onClick={loadDashboard} className="px-3 py-1.5 text-xs text-vault-text-secondary bg-vault-bg-tertiary rounded hover:bg-vault-bg-hover transition-colors">
          Retry
        </button>
      </div>
    );
  }

  if (!vaultStatus) return null;

  const vaultName = vaultStatus.path.split('/').pop() || 'AgentVault';

  return (
    <div className="h-full flex flex-col">
      {/* Header */}
      <div className="border-b border-vault-border px-6 py-4">
        <h1 className="text-lg font-semibold text-vault-text-primary">Dashboard</h1>
        <p className="text-sm text-vault-text-muted mt-1">{vaultName}</p>
      </div>

      <div className="flex-1 overflow-y-auto px-6 py-4 space-y-6">
        {/* Stats row */}
        <div className="grid grid-cols-2 lg:grid-cols-3 gap-3">
          <StatCard
            icon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M19.5 14.25v-2.625a3.375 3.375 0 0 0-3.375-3.375h-1.5A1.125 1.125 0 0 1 13.5 7.125v-1.5a3.375 3.375 0 0 0-3.375-3.375H8.25m2.25 0H5.625c-.621 0-1.125.504-1.125 1.125v17.25c0 .621.504 1.125 1.125 1.125h12.75c.621 0 1.125-.504 1.125-1.125V11.25a9 9 0 0 0-9-9Z" />
              </svg>
            }
            label="Notes"
            value={vaultStatus.noteCount}
          />
          <StatCard
            icon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M2.25 12.75V12A2.25 2.25 0 0 1 4.5 9.75h15A2.25 2.25 0 0 1 21.75 12v.75m-8.69-6.44-2.12-2.12a1.5 1.5 0 0 0-1.061-.44H4.5A2.25 2.25 0 0 0 2.25 6v12a2.25 2.25 0 0 0 2.25 2.25h15A2.25 2.25 0 0 0 21.75 18V9a2.25 2.25 0 0 0-2.25-2.25h-5.379a1.5 1.5 0 0 1-1.06-.44Z" />
              </svg>
            }
            label="Vault"
            value={vaultName}
            mono
          />
          <StatCard
            icon={
              <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={1.5}>
                <path strokeLinecap="round" strokeLinejoin="round" d="M12 6v6h4.5m4.5 0a9 9 0 1 1-18 0 9 9 0 0 1 18 0Z" />
              </svg>
            }
            label="Version"
            value={vaultStatus.version || '0.1.0'}
            mono
          />
        </div>

        {/* Type breakdown */}
        <Section title="Note Types">
          <div className="flex flex-wrap gap-2">
            {NOTE_TYPES.map((t) => (
              <span key={t} className={`type-badge ${typeBadgeClass(t)}`}>
                {t} ({typeCounts[t] || 0})
              </span>
            ))}
          </div>
        </Section>

        {/* Recent notes */}
        <Section title="Recent Notes">
          <NoteList notes={recentNotes} navigate={navigate} />
        </Section>

        {/* Stale notes */}
        {staleNotes.length > 0 && (
          <Section title="Stale Notes" subtitle="Oldest modified">
            <NoteList notes={staleNotes} navigate={navigate} />
          </Section>
        )}

        {/* Index */}
        <Section title="Index">
          <button
            onClick={handleReindex}
            className="flex items-center gap-2 px-3 py-1.5 text-xs text-vault-accent bg-vault-accent-muted rounded-md hover:bg-vault-accent/20 transition-colors"
          >
            <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24" strokeWidth={2}>
              <path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0 3.181 3.183a8.25 8.25 0 0 0 13.803-3.7M4.031 9.865a8.25 8.25 0 0 1 13.803-3.7l3.181 3.182" />
            </svg>
            Re-index vault
          </button>
        </Section>
      </div>
    </div>
  );
};

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
    <div className="bg-vault-bg-secondary border border-vault-border rounded-lg p-3">
      <div className="flex items-center gap-2 text-vault-text-muted mb-1">
        {icon}
        <span className="text-xs uppercase tracking-wide">{label}</span>
      </div>
      <p className={`text-lg font-semibold text-vault-text-primary truncate ${mono ? 'font-mono text-sm' : ''}`}>
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
        <h2 className="text-sm font-semibold text-vault-text-primary uppercase tracking-wide">{title}</h2>
        {subtitle && <span className="text-xs text-vault-text-muted">{subtitle}</span>}
      </div>
      {children}
    </div>
  );
}

function NoteList({
  notes,
  navigate,
}: {
  notes: SearchResult[];
  navigate: ReturnType<typeof useNavigate>;
}) {
  if (notes.length === 0) {
    return <p className="text-sm text-vault-text-muted">No notes yet.</p>;
  }

  return (
    <div className="space-y-1">
      {notes.map((note) => (
        <button
          key={note.id}
          onClick={() => navigate(`/note/${encodeURIComponent(note.id)}`)}
          className="w-full text-left flex items-center gap-3 px-3 py-2 rounded-md hover:bg-vault-bg-hover transition-colors group"
        >
          <span className={`type-badge ${typeBadgeClass(note.type)}`}>
            {note.type}
          </span>
          <span className="text-sm text-vault-text-primary group-hover:text-vault-accent transition-colors truncate flex-1">
            {note.title}
          </span>
          {note.updatedAt && (
            <span className="text-xs text-vault-text-muted flex-shrink-0">
              {formatRelative(note.updatedAt)}
            </span>
          )}
        </button>
      ))}
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

export default DashboardView;
