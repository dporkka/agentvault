import { useState, useEffect, useCallback } from 'react';
import { CheckCircle, AlertTriangle, Clock, FolderTree } from './Icons';
import EmptyState from './EmptyState';
import type { SearchResult } from '../types';

interface Props {
  onOpenNote: (path: string) => void;
}

export default function DecisionDashboard({ onOpenNote }: Props) {
  const [decisions, setDecisions] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(true);

  const loadDecisions = useCallback(async () => {
    try {
      const results = await window.go.main.NoteService.Search('', 'decision', '', false, 0.5, 0);
      setDecisions(results);
    } catch (err) {
      console.error('Failed to load decisions:', err);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadDecisions();
  }, [loadDecisions]);

  const getStatusIcon = (status: string) => {
    switch (status) {
      case 'active': return <CheckCircle className="w-4 h-4 text-success" />;
      case 'superseded': return <Clock className="w-4 h-4 text-warning" />;
      default: return <AlertTriangle className="w-4 h-4 text-text-muted" />;
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="text-sm text-text-muted">Loading decisions...</div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full bg-bg-primary">
      {/* Header */}
      <div className="px-6 py-4 border-b border-border bg-bg-secondary">
        <h1 className="text-lg font-semibold text-text-primary">Decisions</h1>
        <p className="text-xs text-text-muted mt-0.5">
          {decisions.length} decision records
        </p>
      </div>

      {/* Decisions List */}
      <div className="flex-1 overflow-auto p-4">
        {decisions.length === 0 ? (
          <EmptyState
            icon={<FolderTree className="w-12 h-12" />}
            title="No decisions yet"
            description="Create decision records to track important choices"
          />
        ) : (
          <div className="max-w-3xl space-y-2">
            {decisions.map(decision => (
              <button
                key={decision.id}
                onClick={() => onOpenNote(decision.path)}
                className="w-full flex items-start gap-3 px-4 py-3 bg-bg-secondary rounded-lg border border-border hover:border-accent/50 hover:bg-bg-hover transition-all text-left"
              >
                {getStatusIcon(decision.status || 'active')}
                <div className="flex-1 min-w-0">
                  <div className="text-sm font-medium text-text-primary truncate">
                    {decision.title}
                  </div>
                  <div className="flex items-center gap-2 mt-1">
                    {decision.project && (
                      <span className="text-[10px] px-1.5 py-0.5 rounded bg-purple-500/20 text-purple-400">
                        {decision.project}
                      </span>
                    )}
                    <span className="text-[10px] px-1.5 py-0.5 rounded bg-bg-tertiary text-text-muted">
                      {decision.status || 'active'}
                    </span>
                    <span className="text-[10px] text-text-muted">
                      {decision.path}
                    </span>
                  </div>
                  {decision.snippet && (
                    <p className="text-xs text-text-muted mt-1.5 line-clamp-2">
                      {decision.snippet}
                    </p>
                  )}
                </div>
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
