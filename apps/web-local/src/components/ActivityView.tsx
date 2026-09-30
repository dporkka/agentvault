import React, { useEffect, useMemo, useState } from 'react';
import type {
  AgentSession,
  KnowledgeObject,
  MemoryCandidate,
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
  const [mode, setMode] = useState<'activity' | 'review'>('activity');
  const [projects, setProjects] = useState<string[]>([]);
  const [project, setProject] = useState('');
  const [kind, setKind] = useState<'' | TimelineKind>('');
  const [items, setItems] = useState<TimelineItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<TimelineItem | null>(null);

  const [candidates, setCandidates] = useState<MemoryCandidate[]>([]);
  const [candidateLoading, setCandidateLoading] = useState(false);
  const [candidateError, setCandidateError] = useState<string | null>(null);
  const [selectedCandidate, setSelectedCandidate] = useState<MemoryCandidate | null>(null);
  const [reviewer, setReviewer] = useState('');
  const [reviewReason, setReviewReason] = useState('');
  const [reviewing, setReviewing] = useState(false);
  const [reviewMessage, setReviewMessage] = useState<string | null>(null);
  const [reviewError, setReviewError] = useState<string | null>(null);

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
    if (mode !== 'activity') return undefined;

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
  }, [timelineFilter, mode]);

  useEffect(() => {
    if (mode !== 'review') return undefined;

    let cancelled = false;
    setCandidateLoading(true);
    setCandidateError(null);
    setReviewMessage(null);

    knowledgeApi.listMemoryCandidates({ status: 'pending', limit: 100 })
      .then((result) => {
        if (cancelled) return;
        setCandidates(result);
        setSelectedCandidate((current) => {
          if (!current) return null;
          return result.find((candidate) => candidate.id === current.id) ?? null;
        });
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        setCandidates([]);
        setSelectedCandidate(null);
        setCandidateError(err instanceof Error ? err.message : 'Unable to load memory review queue');
      })
      .finally(() => {
        if (!cancelled) setCandidateLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [mode]);

  useEffect(() => {
    if (mode !== 'activity') return undefined;

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
  }, [selected, mode]);

  useEffect(() => {
    if (mode !== 'review') return undefined;

    let cancelled = false;
    setProvenance(null);
    setSession(null);
    setObjects([]);
    setMutation(null);
    setDetailError(null);
    setReviewError(null);
    setReviewer('');
    setReviewReason('');

    if (!selectedCandidate) {
      setDetailLoading(false);
      return () => {
        cancelled = true;
      };
    }

    const requests: Promise<void>[] = [];
    setDetailLoading(true);

    if (selectedCandidate.provenanceId) {
      requests.push(
        knowledgeApi.getProvenance(selectedCandidate.provenanceId)
          .then((value) => {
            if (!cancelled) setProvenance(value);
          }),
      );
    }
    if (selectedCandidate.scopeType === 'session') {
      requests.push(
        knowledgeApi.getSession(selectedCandidate.scopeId)
          .then((value) => {
            if (!cancelled) setSession(value);
          }),
      );
    }
    if (selectedCandidate.objectId) {
      requests.push(
        knowledgeApi.getObject(selectedCandidate.objectId)
          .then((value) => {
            if (!cancelled) setObjects([value]);
          }),
      );
    }

    Promise.all(requests)
      .catch((err: unknown) => {
        if (!cancelled) {
          setDetailError(err instanceof Error ? err.message : 'Unable to load candidate evidence');
        }
      })
      .finally(() => {
        if (!cancelled) setDetailLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [selectedCandidate, mode]);

  const reviewCandidate = async (action: 'accept' | 'reject') => {
    if (!selectedCandidate || !reviewer.trim() || reviewing) return;

    setReviewing(true);
    setReviewError(null);
    setReviewMessage(null);
    const request = {
      reviewedBy: reviewer.trim(),
      reason: reviewReason.trim() || undefined,
    };

    try {
      if (action === 'accept') {
        await knowledgeApi.acceptMemoryCandidate(selectedCandidate.id, request);
        setReviewMessage('Candidate accepted.');
      } else {
        await knowledgeApi.rejectMemoryCandidate(selectedCandidate.id, request);
        setReviewMessage('Candidate rejected.');
      }
      setCandidates((current) => current.filter((candidate) => candidate.id !== selectedCandidate.id));
      setSelectedCandidate(null);
    } catch (err: unknown) {
      setReviewError(err instanceof Error ? err.message : 'Unable to review candidate');
    } finally {
      setReviewing(false);
    }
  };

  const showActivity = () => {
    setMode('activity');
    setSelectedCandidate(null);
    setReviewMessage(null);
    setReviewError(null);
  };

  const showReview = () => {
    setMode('review');
    setSelected(null);
    setReviewMessage(null);
    setReviewError(null);
  };

  return (
    <div className="h-full flex flex-col">
      <div className="border-b border-vault-border px-6 py-4">
        <div className="flex flex-col gap-4 xl:flex-row xl:items-end xl:justify-between">
          <div>
            <h1 className="text-lg font-semibold text-vault-text-primary">Activity</h1>
            <p className="mt-1 text-xs text-vault-text-muted">
              Durable activity and human-reviewed semantic memory in one workspace.
            </p>
            <div className="mt-3 inline-flex rounded-lg border border-vault-border bg-vault-bg-tertiary p-1">
              <button
                type="button"
                onClick={showActivity}
                className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                  mode === 'activity'
                    ? 'bg-vault-accent text-white'
                    : 'text-vault-text-secondary hover:text-vault-text-primary'
                }`}
              >
                Activity timeline
              </button>
              <button
                type="button"
                onClick={showReview}
                className={`rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                  mode === 'review'
                    ? 'bg-vault-accent text-white'
                    : 'text-vault-text-secondary hover:text-vault-text-primary'
                }`}
              >
                Memory review
              </button>
            </div>
          </div>

          {mode === 'activity' && (
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
          )}
        </div>

        {mode === 'activity' ? (
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
        ) : (
          <p className="mt-4 text-xs text-vault-text-muted">
            Pending semantic candidates stay out of durable memory until a reviewer explicitly accepts them.
          </p>
        )}
      </div>

      <div className={`flex min-h-0 flex-1 ${mode === 'review' ? 'flex-col lg:flex-row' : ''}`}>
        <section
          className="min-w-0 flex-1 overflow-y-auto px-4 py-3 sm:px-6"
          aria-label={mode === 'activity' ? 'Activity timeline' : 'Memory review queue'}
        >
          {mode === 'activity' && (
            <>
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
            </>
          )}

          {mode === 'review' && (
            <>
              {reviewMessage && (
                <div className="mb-3 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-400">
                  {reviewMessage}
                </div>
              )}

              {candidateLoading && (
                <div className="flex h-full items-center justify-center text-sm text-vault-text-muted">
                  Loading memory candidates…
                </div>
              )}

              {!candidateLoading && candidateError && (
                <div className="rounded-lg border border-vault-error/30 bg-vault-error/10 px-4 py-3 text-sm text-vault-error">
                  {candidateError}
                </div>
              )}

              {!candidateLoading && !candidateError && candidates.length === 0 && (
                <div className="flex h-full items-center justify-center text-sm text-vault-text-muted">
                  No pending memory candidates.
                </div>
              )}

              {!candidateLoading && !candidateError && candidates.map((candidate) => {
                const active = selectedCandidate?.id === candidate.id;
                return (
                  <button
                    key={candidate.id}
                    type="button"
                    onClick={() => setSelectedCandidate(candidate)}
                    aria-label={`memory candidate: ${candidate.content}`}
                    className={`mb-2 w-full rounded-xl border p-3 text-left transition-colors ${
                      active
                        ? 'border-vault-accent bg-vault-accent-muted'
                        : 'border-vault-border bg-vault-bg-secondary hover:bg-vault-bg-hover'
                    }`}
                  >
                    <div className="flex items-start gap-3">
                      <div className="mt-1 h-2 w-2 shrink-0 rounded-full bg-amber-400" />
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="text-[10px] font-semibold uppercase tracking-wide text-vault-accent">
                            {candidate.memoryKind}
                          </span>
                          <span className="text-[10px] text-vault-text-muted">
                            {candidate.scopeType} · {candidate.scopeId}
                          </span>
                        </div>
                        <p className="mt-1 text-sm font-medium text-vault-text-primary">{candidate.content}</p>
                        <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-vault-text-muted">
                          <span>{Math.round(candidate.confidence * 100)}% confidence</span>
                          {candidate.proposedBy && <span>proposed by {candidate.proposedBy}</span>}
                          <span>{displayTime(candidate.createdAt)}</span>
                        </div>
                      </div>
                    </div>
                  </button>
                );
              })}
            </>
          )}
        </section>

        <aside
          className={mode === 'activity'
            ? 'hidden w-[23rem] shrink-0 overflow-y-auto border-l border-vault-border bg-vault-bg-secondary/40 p-4 lg:block'
            : 'w-full shrink-0 overflow-y-auto border-t border-vault-border bg-vault-bg-secondary/40 p-4 lg:w-[23rem] lg:border-l lg:border-t-0'}
          aria-label={mode === 'activity' ? 'Activity details' : 'Memory review details'}
        >
          {mode === 'activity' && !selected && (
            <div className="flex h-full items-center justify-center text-center text-xs text-vault-text-muted">
              Select an activity item to inspect its durable context.
            </div>
          )}

          {mode === 'review' && !selectedCandidate && (
            <div className="flex h-full items-center justify-center text-center text-xs text-vault-text-muted">
              Select a candidate to inspect its evidence before review.
            </div>
          )}

          {mode === 'activity' && selected && (
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
                      <div className="mt-3 space-y-2 border-t border-vault-border pt-2">
                        {provenance.evidence?.map((evidence, index) => (
                          <div key={`${evidence.source}:${evidence.id ?? evidence.path ?? index}`} className="text-[10px] text-vault-text-secondary">
                            <p className="break-all">{evidence.path || evidence.id || evidence.source}</p>
                            {evidence.quote && <p className="mt-1 text-vault-text-muted">{evidence.quote}</p>}
                          </div>
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

          {mode === 'review' && selectedCandidate && (
            <div className="space-y-5">
              <div>
                <span className="text-[10px] font-semibold uppercase tracking-wide text-vault-accent">
                  {selectedCandidate.memoryKind}
                </span>
                <h2 className="mt-1 text-sm font-semibold text-vault-text-primary">{selectedCandidate.content}</h2>
                <p className="mt-1 break-all font-mono text-[10px] text-vault-text-muted">{selectedCandidate.id}</p>
              </div>

              {detailLoading && <p className="text-xs text-vault-text-muted">Loading candidate evidence…</p>}
              {detailError && <p className="text-xs text-vault-error">{detailError}</p>}

              <section>
                <h3 className="text-xs font-semibold text-vault-text-secondary">Source episode</h3>
                <div className="mt-2 rounded-lg border border-vault-border bg-vault-bg-tertiary p-3 text-xs">
                  <p className="break-all font-mono text-[10px] text-vault-text-primary">{selectedCandidate.sourceEpisodeId}</p>
                  <p className="mt-2 text-vault-text-muted">
                    {selectedCandidate.scopeType} · {selectedCandidate.scopeId}
                  </p>
                  <p className="mt-1 text-vault-text-muted">
                    {Math.round(selectedCandidate.confidence * 100)}% confidence
                  </p>
                  {selectedCandidate.proposedBy && (
                    <p className="mt-1 text-vault-text-muted">proposed by {selectedCandidate.proposedBy}</p>
                  )}
                </div>
              </section>

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
                  <h3 className="text-xs font-semibold text-vault-text-secondary">Evidence</h3>
                  <div className="mt-2 rounded-lg border border-vault-border bg-vault-bg-tertiary p-3 text-xs">
                    <div className="flex items-center justify-between gap-3">
                      <span className="text-vault-text-primary">{provenance.sourceType}</span>
                      <span className="text-vault-text-muted">{Math.round(provenance.confidence * 100)}%</span>
                    </div>
                    <p className="mt-1 text-[10px] text-vault-text-muted">
                      observed {displayTime(provenance.observedAt)}
                    </p>
                    {(provenance.evidence ?? []).length > 0 && (
                      <div className="mt-3 space-y-2 border-t border-vault-border pt-2">
                        {provenance.evidence?.map((evidence, index) => (
                          <div key={`${evidence.source}:${evidence.id ?? evidence.path ?? index}`} className="text-[10px] text-vault-text-secondary">
                            <p className="break-all">{evidence.path || evidence.id || evidence.source}</p>
                            {evidence.quote && (
                              <p className="mt-1 rounded bg-vault-bg-secondary p-2 text-vault-text-primary">{evidence.quote}</p>
                            )}
                          </div>
                        ))}
                      </div>
                    )}
                  </div>
                </section>
              )}

              <section>
                <h3 className="text-xs font-semibold text-vault-text-secondary">Review</h3>
                <div className="mt-2 space-y-3 rounded-lg border border-vault-border bg-vault-bg-tertiary p-3">
                  <label className="block text-xs text-vault-text-secondary">
                    <span>Reviewer</span>
                    <input
                      aria-label="Reviewer"
                      value={reviewer}
                      onChange={(event) => setReviewer(event.target.value)}
                      placeholder="Your name or reviewer ID"
                      className="mt-1 w-full rounded-md border border-vault-border bg-vault-bg-secondary px-2.5 py-2 text-xs text-vault-text-primary outline-none focus:border-vault-accent"
                    />
                  </label>
                  <label className="block text-xs text-vault-text-secondary">
                    <span>Review reason</span>
                    <textarea
                      aria-label="Review reason"
                      value={reviewReason}
                      onChange={(event) => setReviewReason(event.target.value)}
                      placeholder="Optional evidence or rationale"
                      rows={3}
                      className="mt-1 w-full resize-none rounded-md border border-vault-border bg-vault-bg-secondary px-2.5 py-2 text-xs text-vault-text-primary outline-none focus:border-vault-accent"
                    />
                  </label>

                  {reviewError && <p className="text-xs text-vault-error">{reviewError}</p>}

                  <div className="flex gap-2">
                    <button
                      type="button"
                      disabled={!reviewer.trim() || reviewing}
                      onClick={() => void reviewCandidate('accept')}
                      className="flex-1 rounded-md bg-vault-accent px-3 py-2 text-xs font-semibold text-white disabled:cursor-not-allowed disabled:opacity-40"
                    >
                      {reviewing ? 'Reviewing…' : 'Accept memory'}
                    </button>
                    <button
                      type="button"
                      disabled={!reviewer.trim() || reviewing}
                      onClick={() => void reviewCandidate('reject')}
                      className="flex-1 rounded-md border border-vault-error/40 px-3 py-2 text-xs font-semibold text-vault-error hover:bg-vault-error/10 disabled:cursor-not-allowed disabled:opacity-40"
                    >
                      Reject candidate
                    </button>
                  </div>
                </div>
              </section>
            </div>
          )}
        </aside>
      </div>
    </div>
  );
};

export default ActivityView;
