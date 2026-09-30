import React, { useEffect, useMemo, useState } from 'react';
import type {
  AgentSession,
  KnowledgeObject,
  MutationProposal,
  ProvenanceRecord,
  TimelineFilter,
  TimelineItem,
  TimelineKind,
} from '@agentvault/contract';
import { api, knowledgeApi } from '@/api/client';

const KIND_FILTERS: Array<{ label: string; value: '' | TimelineKind }> = [
  { label: 'All activity', value: '' },
  { label: 'Captures', value: 'capture' },
  { label: 'Session events', value: 'session_event' },
  { label: 'Episodes', value: 'episode' },
  { label: 'Memories', value: 'memory' },
  { label: 'Mutations', value: 'mutation' },
];

function displayTime(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return parsed.toLocaleString();
}

function kindLabel(kind: TimelineKind): string {
  return kind.replace('_', ' ');
}

const ActivityView: React.FC = () => {
  const [projects, setProjects] = useState<string[]>([]);
  const [project, setProject] = useState('');
  const [kind, setKind] = useState<'' | TimelineKind>('');
  const [items, setItems] = useState<TimelineItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<TimelineItem | null>(null);

  const [provenance, setProvenance] = useState<ProvenanceRecord | null>(null);
  const [session, setSession] = useState<AgentSession | null>(null);
  const [objects, setObjects] = useState<KnowledgeObject[]>([]);
  const [mutation, setMutation] = useState<MutationProposal | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [detailError, setDetailError] = useState<string | null>(null);

  const timelineFilter = useMemo<TimelineFilter>(() => ({
    ...(project ? { project } : {}),
    ...(kind ? { kind } : {}),
    limit: 100,
  }), [project, kind]);

  useEffect(() => {
    let cancelled = false;
    api.getProjects()
      .then((result) => {
        if (!cancelled) setProjects(result);
      })
      .catch(() => {
        if (!cancelled) setProjects([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);

    knowledgeApi.listTimeline(timelineFilter)
      .then((result) => {
        if (cancelled) return;
        setItems(result);
        setSelected((current) => {
          if (!current) return null;
          return result.find((item) => item.id === current.id && item.kind === current.kind) ?? null;
        });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setItems([]);
        setError(err instanceof Error ? err.message : 'Unable to load activity');
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [timelineFilter]);

  useEffect(() => {
    let cancelled = false;
    setProvenance(null);
    setSession(null);
    setObjects([]);
    setMutation(null);
    setDetailError(null);

    if (!selected) {
      setDetailLoading(false);
      return () => {
        cancelled = true;
      };
    }

    const requests: Promise<void>[] = [];
    setDetailLoading(true);

    if (selected.provenanceId) {
      requests.push(
        knowledgeApi.getProvenance(selected.provenanceId)
          .then((value) => {
            if (!cancelled) setProvenance(value);
          }),
      );
    }
    if (selected.sessionId) {
      requests.push(
        knowledgeApi.getSession(selected.sessionId)
          .then((value) => {
            if (!cancelled) setSession(value);
          }),
      );
    }
    for (const objectId of selected.objectIds ?? []) {
      requests.push(
        knowledgeApi.getObject(objectId)
          .then((value) => {
            if (!cancelled) {
              setObjects((current) => (
                current.some((object) => object.id === value.id)
                  ? current
                  : [...current, value]
              ));
            }
          }),
      );
    }
    if (selected.kind === 'mutation') {
      requests.push(
        knowledgeApi.getMutation(selected.id)
          .then((value) => {
            if (!cancelled) setMutation(value);
          }),
      );
    }

    Promise.all(requests)
      .catch((err: unknown) => {
        if (!cancelled) {
          setDetailError(err instanceof Error ? err.message : 'Unable to load activity details');
        }
      })
      .finally(() => {
        if (!cancelled) setDetailLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [selected]);

  return (
    <div className="h-full flex flex-col">
      <div className="border-b border-vault-border px-6 py-4">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h1 className="text-lg font-semibold text-vault-text-primary">Activity</h1>
            <p className="mt-1 text-xs text-vault-text-muted">
              Captures, agent execution, memory, episodes, and reviewable mutations in one timeline.
            </p>
          </div>

          <label className="flex min-w-52 flex-col gap-1 text-xs text-vault-text-secondary">
            <span>Project</span>
            <select
              aria-label="Project"
              value={project}
              onChange={(event) => setProject(event.target.value)}
              className="rounded-lg border border-vault-border bg-vault-bg-tertiary px-3 py-2 text-sm text-vault-text-primary outline-none focus:border-vault-accent"
            >
              <option value="">All projects</option>
              {projects.map((value) => (
                <option key={value} value={value}>{value}</option>
              ))}
            </select>
          </label>
        </div>

        <div className="mt-4 flex flex-wrap gap-1.5">
          {KIND_FILTERS.map((filter) => (
            <button
              key={filter.label}
              type="button"
              onClick={() => setKind(filter.value)}
              className={`rounded-full px-3 py-1 text-xs font-medium transition-colors ${
                kind === filter.value
                  ? 'bg-vault-accent text-white'
                  : 'bg-vault-bg-tertiary text-vault-text-secondary hover:bg-vault-bg-hover hover:text-vault-text-primary'
              }`}
            >
              {filter.label}
            </button>
          ))}
        </div>
      </div>

      <div className="flex min-h-0 flex-1">
        <section className="min-w-0 flex-1 overflow-y-auto px-4 py-3 sm:px-6" aria-label="Activity timeline">
          {loading && (
            <div className="flex h-full items-center justify-center text-sm text-vault-text-muted">
              Loading activity…
            </div>
          )}

          {!loading && error && (
            <div className="rounded-lg border border-vault-error/30 bg-vault-error/10 px-4 py-3 text-sm text-vault-error">
              {error}
            </div>
          )}

          {!loading && !error && items.length === 0 && (
            <div className="flex h-full items-center justify-center text-sm text-vault-text-muted">
              No activity matches these filters.
            </div>
          )}

          {!loading && !error && items.map((item) => {
            const active = selected?.id === item.id && selected?.kind === item.kind;
            const summary = item.summary || item.title || item.eventType || item.id;
            return (
              <button
                key={`${item.kind}:${item.id}`}
                type="button"
                onClick={() => setSelected(item)}
                aria-label={`${kindLabel(item.kind)}: ${summary}`}
                className={`mb-2 w-full rounded-xl border p-3 text-left transition-colors ${
                  active
                    ? 'border-vault-accent bg-vault-accent-muted'
                    : 'border-vault-border bg-vault-bg-secondary hover:bg-vault-bg-hover'
                }`}
              >
                <div className="flex items-start gap-3">
                  <div className="mt-1 h-2 w-2 shrink-0 rounded-full bg-vault-accent" />
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="text-[10px] font-semibold uppercase tracking-wide text-vault-accent">
                        {kindLabel(item.kind)}
                      </span>
                      {item.eventType && (
                        <span className="text-[10px] text-vault-text-muted">{item.eventType}</span>
                      )}
                      {item.project && (
                        <span className="text-[10px] text-vault-text-muted">{item.project}</span>
                      )}
                    </div>
                    <p className="mt-1 text-sm font-medium text-vault-text-primary">{summary}</p>
                    <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-vault-text-muted">
                      <span>{displayTime(item.occurredAt)}</span>
                      {item.agentId && <span>agent: {item.agentId}</span>}
                      {item.sessionId && <span>session: {item.sessionId}</span>}
                    </div>
                  </div>
                </div>
              </button>
            );
          })}
        </section>

        <aside className="hidden w-[23rem] shrink-0 overflow-y-auto border-l border-vault-border bg-vault-bg-secondary/40 p-4 lg:block" aria-label="Activity details">
          {!selected && (
            <div className="flex h-full items-center justify-center text-center text-xs text-vault-text-muted">
              Select an activity item to inspect its durable context.
            </div>
          )}

          {selected && (
            <div className="space-y-5">
              <div>
                <span className="text-[10px] font-semibold uppercase tracking-wide text-vault-accent">
                  {kindLabel(selected.kind)}
                </span>
                <h2 className="mt-1 text-sm font-semibold text-vault-text-primary">
                  {selected.summary || selected.title || selected.eventType || selected.id}
                </h2>
                <p className="mt-1 break-all font-mono text-[10px] text-vault-text-muted">{selected.id}</p>
              </div>

              {detailLoading && <p className="text-xs text-vault-text-muted">Loading durable context…</p>}
              {detailError && <p className="text-xs text-vault-error">{detailError}</p>}

              {mutation && (
                <section>
                  <h3 className="text-xs font-semibold text-vault-text-secondary">Mutation</h3>
                  <div className="mt-2 rounded-lg border border-vault-border bg-vault-bg-tertiary p-3">
                    <div className="flex items-center justify-between gap-3">
                      <span className="font-mono text-[11px] text-vault-text-primary">{mutation.path}</span>
                      <span className="rounded-full bg-vault-accent-muted px-2 py-0.5 text-[10px] text-vault-accent">
                        {mutation.status}
                      </span>
                    </div>
                    {mutation.diff && (
                      <pre className="mt-3 overflow-x-auto whitespace-pre-wrap text-[10px] text-vault-text-secondary">{mutation.diff}</pre>
                    )}
                  </div>
                </section>
              )}

              {session && (
                <section>
                  <h3 className="text-xs font-semibold text-vault-text-secondary">Session</h3>
                  <div className="mt-2 rounded-lg border border-vault-border bg-vault-bg-tertiary p-3 text-xs">
                    <p className="text-vault-text-primary">{session.objective}</p>
                    <p className="mt-1 text-vault-text-muted">{session.agentId} · {session.status}</p>
                  </div>
                </section>
              )}

              {objects.length > 0 && (
                <section>
                  <h3 className="text-xs font-semibold text-vault-text-secondary">Objects</h3>
                  <div className="mt-2 space-y-2">
                    {objects.map((object) => (
                      <div key={object.id} className="rounded-lg border border-vault-border bg-vault-bg-tertiary p-3">
                        <p className="text-xs font-medium text-vault-text-primary">{object.title}</p>
                        <p className="mt-1 text-[10px] text-vault-text-muted">{object.type} · {object.id}</p>
                      </div>
                    ))}
                  </div>
                </section>
              )}

              {provenance && (
                <section>
                  <h3 className="text-xs font-semibold text-vault-text-secondary">Provenance</h3>
                  <div className="mt-2 rounded-lg border border-vault-border bg-vault-bg-tertiary p-3 text-xs">
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-vault-text-primary">{provenance.sourceType}</span>
                      <span className="text-vault-text-muted">{Math.round(provenance.confidence * 100)}%</span>
                    </div>
                    <p className="mt-1 text-[10px] text-vault-text-muted">
                      observed {displayTime(provenance.observedAt)}
                    </p>
                    {(provenance.evidence ?? []).length > 0 && (
                      <div className="mt-3 space-y-1 border-t border-vault-border pt-2">
                        {provenance.evidence?.map((evidence, index) => (
                          <p key={`${evidence.source}:${evidence.id ?? evidence.path ?? index}`} className="break-all text-[10px] text-vault-text-secondary">
                            {evidence.path || evidence.id || evidence.source}
                          </p>
                        ))}
                      </div>
                    )}
                  </div>
                </section>
              )}

              {selected.metadata && Object.keys(selected.metadata).length > 0 && (
                <section>
                  <h3 className="text-xs font-semibold text-vault-text-secondary">Metadata</h3>
                  <pre className="mt-2 overflow-x-auto whitespace-pre-wrap rounded-lg border border-vault-border bg-vault-bg-tertiary p-3 text-[10px] text-vault-text-secondary">
                    {JSON.stringify(selected.metadata, null, 2)}
                  </pre>
                </section>
              )}
            </div>
          )}
        </aside>
      </div>
    </div>
  );
};

export default ActivityView;
