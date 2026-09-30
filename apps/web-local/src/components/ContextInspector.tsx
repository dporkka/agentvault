import React, { useEffect, useMemo, useState } from 'react';
import { api } from '@/api/client';
import type { ContextBundle, ContextItem } from '@agentvault/contract';

const DEFAULT_TOKEN_BUDGET = 8000;

function ItemCard({ item, index }: { item: ContextItem; index: number }) {
  const provenance = item.provenance;
  return (
    <article className="rounded-xl border border-vault-border bg-vault-bg-secondary p-4">
      <div className="flex flex-wrap items-center gap-2 mb-2">
        <span className="text-xs font-mono text-vault-text-muted">#{index + 1}</span>
        <span className="px-2 py-0.5 rounded-full bg-vault-accent-muted text-vault-accent text-xs font-medium">
          {item.kind}
        </span>
        <span className="text-xs text-vault-text-muted">Score {item.score.toFixed(3)}</span>
        <span className="text-xs text-vault-text-muted">{item.estimatedTokens} tokens</span>
      </div>
      <h3 className="text-sm font-semibold text-vault-text-primary">{item.title || item.id}</h3>
      {item.path && <p className="text-xs font-mono text-vault-text-muted mt-1">{item.path}</p>}
      <p className="text-sm text-vault-text-secondary whitespace-pre-wrap mt-3 leading-relaxed">{item.content}</p>
      {provenance && (
        <div className="mt-3 pt-3 border-t border-vault-border text-xs text-vault-text-muted">
          <span>{provenance.sourceType} · {Math.round(provenance.confidence * 100)}% confidence</span>
          {provenance.sourceId && <span> · source {provenance.sourceId}</span>}
          {provenance.evidence?.length ? <span> · {provenance.evidence.length} evidence item{provenance.evidence.length === 1 ? '' : 's'}</span> : null}
        </div>
      )}
    </article>
  );
}

const ContextInspector: React.FC = () => {
  const [task, setTask] = useState('');
  const [project, setProject] = useState('');
  const [projects, setProjects] = useState<string[]>([]);
  const [tokenBudget, setTokenBudget] = useState(DEFAULT_TOKEN_BUDGET);
  const [bundle, setBundle] = useState<ContextBundle | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    api.getProjects().then(setProjects).catch(() => setProjects([]));
  }, []);

  const usagePercent = useMemo(() => {
    if (!bundle || bundle.tokenBudget <= 0) return 0;
    return Math.min(100, Math.round((bundle.estimatedTokens / bundle.tokenBudget) * 100));
  }, [bundle]);

  const handleCompile = async (event: React.FormEvent) => {
    event.preventDefault();
    const trimmed = task.trim();
    if (!trimmed || loading) return;

    setLoading(true);
    setError(null);
    try {
      const result = await api.compileContext({
        task: trimmed,
        project: project || undefined,
        tokenBudget,
      });
      setBundle(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Context compilation failed');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="h-full flex flex-col overflow-hidden">
      <header className="border-b border-vault-border px-6 py-4">
        <h1 className="text-lg font-semibold text-vault-text-primary">Context Inspector</h1>
        <p className="text-xs text-vault-text-muted mt-1">
          Inspect exactly what AgentVault will give an agent, including ranking, provenance, scope, and token cost.
        </p>
      </header>

      <div className="flex-1 overflow-y-auto p-6">
        <form onSubmit={handleCompile} className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_220px_180px_auto] items-end">
          <label className="block">
            <span className="block text-xs font-medium text-vault-text-secondary mb-1.5">Task</span>
            <input
              aria-label="Task"
              value={task}
              onChange={(e) => setTask(e.target.value)}
              placeholder="Describe the agent task to compile context for"
              className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2.5 text-sm text-vault-text-primary outline-none focus:border-vault-accent"
            />
          </label>
          <label className="block">
            <span className="block text-xs font-medium text-vault-text-secondary mb-1.5">Project</span>
            <select
              aria-label="Project"
              value={project}
              onChange={(e) => setProject(e.target.value)}
              className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2.5 text-sm text-vault-text-primary outline-none focus:border-vault-accent"
            >
              <option value="">All / unscoped</option>
              {projects.map((value) => <option key={value} value={value}>{value}</option>)}
            </select>
          </label>
          <label className="block">
            <span className="block text-xs font-medium text-vault-text-secondary mb-1.5">Token budget</span>
            <input
              aria-label="Token budget"
              type="number"
              min={256}
              max={128000}
              value={tokenBudget}
              onChange={(e) => setTokenBudget(Number(e.target.value) || DEFAULT_TOKEN_BUDGET)}
              className="w-full bg-vault-bg-tertiary border border-vault-border rounded-lg px-3 py-2.5 text-sm text-vault-text-primary outline-none focus:border-vault-accent"
            />
          </label>
          <button
            type="submit"
            disabled={!task.trim() || loading}
            className="px-4 py-2.5 rounded-lg bg-vault-accent text-white text-sm font-medium disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {loading ? 'Compiling…' : 'Compile context'}
          </button>
        </form>

        {error && <div className="mt-4 rounded-lg bg-vault-error/10 text-vault-error px-4 py-3 text-sm">{error}</div>}

        {bundle && (
          <section className="mt-6 space-y-5">
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              <div className="rounded-xl border border-vault-border bg-vault-bg-secondary p-4">
                <div className="text-xs text-vault-text-muted">Token usage</div>
                <div className="text-lg font-semibold text-vault-text-primary mt-1">{bundle.estimatedTokens.toLocaleString()} / {bundle.tokenBudget.toLocaleString()} tokens</div>
                <div className="h-1.5 bg-vault-bg-tertiary rounded-full mt-3 overflow-hidden">
                  <div className="h-full bg-vault-accent" style={{ width: usagePercent + '%' }} />
                </div>
              </div>
              <div className="rounded-xl border border-vault-border bg-vault-bg-secondary p-4">
                <div className="text-xs text-vault-text-muted">Selection</div>
                <div className="text-lg font-semibold text-vault-text-primary mt-1">{bundle.stats.included} included</div>
                <div className="text-xs text-vault-text-muted mt-1">{bundle.stats.dropped} dropped of {bundle.stats.candidates} candidates</div>
              </div>
              <div className="rounded-xl border border-vault-border bg-vault-bg-secondary p-4">
                <div className="text-xs text-vault-text-muted">Scope</div>
                <div className="text-sm font-semibold text-vault-text-primary mt-1">{bundle.project || 'Unscoped'}</div>
                <div className="text-xs text-vault-text-muted mt-1">{bundle.agentId || 'No agent'} · {bundle.sessionId || 'No session'}</div>
              </div>
              <div className="rounded-xl border border-vault-border bg-vault-bg-secondary p-4">
                <div className="text-xs text-vault-text-muted">Packing</div>
                <div className="text-sm font-semibold text-vault-text-primary mt-1">{bundle.truncated ? 'Truncated' : 'Complete'}</div>
                <div className="text-xs text-vault-text-muted mt-1">As of {new Date(bundle.asOf).toLocaleString()}</div>
              </div>
            </div>

            <div className="rounded-xl border border-vault-border bg-vault-bg-secondary p-4">
              <div className="text-xs font-medium text-vault-text-muted mb-3">Included by kind</div>
              <div className="flex flex-wrap gap-2">
                {Object.entries(bundle.stats.byKind).map(([kind, count]) => (
                  <span key={kind} className="px-2.5 py-1 rounded-full bg-vault-bg-tertiary text-xs text-vault-text-secondary">
                    {kind}: {count}
                  </span>
                ))}
              </div>
            </div>

            <div className="space-y-3">
              {bundle.items.map((item, index) => <ItemCard key={item.kind + ':' + item.id} item={item} index={index} />)}
            </div>
          </section>
        )}

        {!bundle && !error && (
          <div className="mt-12 text-center text-vault-text-muted">
            <p className="text-sm">Compile a task to see AgentVault's exact context bundle.</p>
          </div>
        )}
      </div>
    </div>
  );
};

export default ContextInspector;
