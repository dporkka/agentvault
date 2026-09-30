import { describe, it, expect, vi, beforeEach } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import ActivityView from '../ActivityView';

const {
  mockGetProjects,
  mockListTimeline,
  mockGetProvenance,
  mockGetSession,
  mockGetObject,
  mockGetMutation,
  mockListMemoryCandidates,
  mockAcceptMemoryCandidate,
  mockRejectMemoryCandidate,
} = vi.hoisted(() => ({
  mockGetProjects: vi.fn(),
  mockListTimeline: vi.fn(),
  mockGetProvenance: vi.fn(),
  mockGetSession: vi.fn(),
  mockGetObject: vi.fn(),
  mockGetMutation: vi.fn(),
  mockListMemoryCandidates: vi.fn(),
  mockAcceptMemoryCandidate: vi.fn(),
  mockRejectMemoryCandidate: vi.fn(),
}));

vi.mock('@/api/client', () => ({
  api: {
    getProjects: mockGetProjects,
  },
  knowledgeApi: {
    listTimeline: mockListTimeline,
    getProvenance: mockGetProvenance,
    getSession: mockGetSession,
    getObject: mockGetObject,
    getMutation: mockGetMutation,
    listMemoryCandidates: mockListMemoryCandidates,
    acceptMemoryCandidate: mockAcceptMemoryCandidate,
    rejectMemoryCandidate: mockRejectMemoryCandidate,
  },
}));

const episode = {
  kind: 'episode' as const,
  id: 'episode_1',
  summary: 'Architecture changed',
  project: 'agentvault',
  agentId: 'architect',
  sessionId: 'session_1',
  eventType: 'architecture.changed',
  objectIds: ['obj_1'],
  provenanceId: 'prov_1',
  occurredAt: '2026-09-30T12:00:00Z',
  createdAt: '2026-09-30T12:01:00Z',
  metadata: {},
};

const mutation = {
  kind: 'mutation' as const,
  id: 'mutation_1',
  title: '10-notes/timeline.md',
  summary: 'Normalize timeline projection',
  project: 'agentvault',
  agentId: 'backend-engineer',
  sessionId: 'session_1',
  eventType: 'replace',
  occurredAt: '2026-09-30T12:02:00Z',
  createdAt: '2026-09-30T12:00:30Z',
  metadata: { status: 'proposed' },
};

describe('ActivityView', () => {
  beforeEach(() => {
    mockGetProjects.mockReset();
    mockListTimeline.mockReset();
    mockGetProvenance.mockReset();
    mockGetSession.mockReset();
    mockGetObject.mockReset();
    mockGetMutation.mockReset();
    mockListMemoryCandidates.mockReset();
    mockAcceptMemoryCandidate.mockReset();
    mockRejectMemoryCandidate.mockReset();

    mockGetProjects.mockResolvedValue(['agentvault', 'adacavo']);
    mockListTimeline.mockResolvedValue([mutation, episode]);
    mockListMemoryCandidates.mockResolvedValue([]);
    mockAcceptMemoryCandidate.mockImplementation(async (id: string, request: { reviewedBy: string; reason?: string }) => ({
      id,
      sourceEpisodeId: 'episode_candidate_1',
      memoryKind: 'decision',
      scopeType: 'project',
      scopeId: 'agentvault',
      content: 'Keep deterministic extraction reviewable.',
      provenanceId: 'prov_candidate_1',
      confidence: 0.91,
      status: 'accepted',
      reviewedBy: request.reviewedBy,
      reviewReason: request.reason,
      resultMemoryId: 'mem_candidate_1',
      createdAt: '2026-09-30T12:10:00Z',
      updatedAt: '2026-09-30T12:20:00Z',
      reviewedAt: '2026-09-30T12:20:00Z',
    }));
    mockRejectMemoryCandidate.mockImplementation(async (id: string, request: { reviewedBy: string; reason?: string }) => ({
      id,
      sourceEpisodeId: 'episode_candidate_1',
      memoryKind: 'decision',
      scopeType: 'project',
      scopeId: 'agentvault',
      content: 'Keep deterministic extraction reviewable.',
      provenanceId: 'prov_candidate_1',
      confidence: 0.91,
      status: 'rejected',
      reviewedBy: request.reviewedBy,
      reviewReason: request.reason,
      createdAt: '2026-09-30T12:10:00Z',
      updatedAt: '2026-09-30T12:20:00Z',
      reviewedAt: '2026-09-30T12:20:00Z',
    }));
  });

  it('loads and renders the unified activity stream', async () => {
    render(<ActivityView />);

    expect(screen.getByRole('heading', { name: 'Activity' })).toBeInTheDocument();
    expect(await screen.findByText('Architecture changed')).toBeInTheDocument();
    expect(screen.getByText('Normalize timeline projection')).toBeInTheDocument();

    expect(mockListTimeline).toHaveBeenCalledWith(
      expect.objectContaining({ limit: 100 }),
    );
  });

  it('refetches when project and kind filters change', async () => {
    render(<ActivityView />);
    await screen.findByText('Architecture changed');

    fireEvent.change(screen.getByLabelText('Project'), {
      target: { value: 'agentvault' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Episodes' }));

    await waitFor(() =>
      expect(mockListTimeline).toHaveBeenLastCalledWith(
        expect.objectContaining({
          project: 'agentvault',
          kind: 'episode',
          limit: 100,
        }),
      ),
    );
  });

  it('loads provenance, session, and object context for a selected item', async () => {
    mockGetProvenance.mockResolvedValue({
      id: 'prov_1',
      sourceType: 'agent-session',
      sourceId: 'source_1',
      confidence: 0.96,
      observedAt: '2026-09-30T11:59:00Z',
      createdAt: '2026-09-30T12:00:00Z',
      evidence: [{ source: 'file', path: 'docs/DESIGN.md' }],
    });
    mockGetSession.mockResolvedValue({
      id: 'session_1',
      agentId: 'architect',
      project: 'agentvault',
      objective: 'Improve AgentVault architecture',
      status: 'active',
      startedAt: '2026-09-30T10:00:00Z',
      updatedAt: '2026-09-30T12:00:00Z',
      events: [],
    });
    mockGetObject.mockResolvedValue({
      id: 'obj_1',
      type: 'project',
      title: 'AgentVault',
      project: 'agentvault',
      createdAt: '2026-09-29T10:00:00Z',
      updatedAt: '2026-09-30T10:00:00Z',
    });

    render(<ActivityView />);
    fireEvent.click(await screen.findByRole('button', { name: /Architecture changed/i }));

    expect(await screen.findByText('agent-session')).toBeInTheDocument();
    expect(screen.getByText('Improve AgentVault architecture')).toBeInTheDocument();
    expect(screen.getByText('AgentVault')).toBeInTheDocument();
    expect(screen.getByText('docs/DESIGN.md')).toBeInTheDocument();

    expect(mockGetProvenance).toHaveBeenCalledWith('prov_1');
    expect(mockGetSession).toHaveBeenCalledWith('session_1');
    expect(mockGetObject).toHaveBeenCalledWith('obj_1');
    expect(mockGetMutation).not.toHaveBeenCalled();
  });

  it('loads mutation state when a mutation is selected', async () => {
    mockGetSession.mockResolvedValue({
      id: 'session_1',
      agentId: 'backend-engineer',
      project: 'agentvault',
      objective: 'Implement timeline UI',
      status: 'active',
      startedAt: '2026-09-30T10:00:00Z',
      updatedAt: '2026-09-30T12:00:00Z',
      events: [],
    });
    mockGetMutation.mockResolvedValue({
      id: 'mutation_1',
      kind: 'replace',
      path: '10-notes/timeline.md',
      reason: 'Normalize timeline projection',
      status: 'proposed',
      beforeExists: true,
      afterExists: true,
      diff: '@@ -1 +1 @@',
      createdAt: '2026-09-30T12:00:30Z',
      updatedAt: '2026-09-30T12:02:00Z',
    });

    render(<ActivityView />);
    fireEvent.click(await screen.findByRole('button', { name: /Normalize timeline projection/i }));

    expect(await screen.findByText('proposed')).toBeInTheDocument();
    expect(screen.getByText('@@ -1 +1 @@')).toBeInTheDocument();
    expect(mockGetMutation).toHaveBeenCalledWith('mutation_1');
  });

  it('shows API errors without hiding the page controls', async () => {
    mockListTimeline.mockRejectedValue(new Error('Timeline unavailable'));

    render(<ActivityView />);

    expect(await screen.findByText('Timeline unavailable')).toBeInTheDocument();
    expect(screen.getByLabelText('Project')).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'All activity' })).toBeInTheDocument();
  });


  it('loads the pending memory review queue without mixing candidates into the activity timeline', async () => {
    mockListMemoryCandidates.mockResolvedValue([{
      id: 'candidate_1',
      sourceEpisodeId: 'episode_candidate_1',
      memoryKind: 'decision',
      scopeType: 'project',
      scopeId: 'agentvault',
      content: 'Keep deterministic extraction reviewable.',
      provenanceId: 'prov_candidate_1',
      confidence: 0.91,
      status: 'pending',
      proposedBy: 'deterministic-extractor-v1',
      createdAt: '2026-09-30T12:10:00Z',
      updatedAt: '2026-09-30T12:10:00Z',
    }]);

    render(<ActivityView />);
    await screen.findByText('Architecture changed');

    fireEvent.click(screen.getByRole('button', { name: 'Memory review' }));

    expect(await screen.findByText('Keep deterministic extraction reviewable.')).toBeInTheDocument();
    expect(screen.getByText('91% confidence')).toBeInTheDocument();
    expect(screen.getByText('project · agentvault')).toBeInTheDocument();
    expect(mockListMemoryCandidates).toHaveBeenCalledWith({ status: 'pending', limit: 100 });
    expect(mockListTimeline).toHaveBeenCalledTimes(1);
  });

  it('shows source episode, provenance evidence, and scoped context for a selected candidate', async () => {
    mockListMemoryCandidates.mockResolvedValue([{
      id: 'candidate_1',
      sourceEpisodeId: 'episode_candidate_1',
      memoryKind: 'constraint',
      scopeType: 'session',
      scopeId: 'session_1',
      content: 'Semantic memory requires explicit review.',
      objectId: 'obj_1',
      provenanceId: 'prov_candidate_1',
      confidence: 0.94,
      status: 'pending',
      proposedBy: 'deterministic-extractor-v1',
      createdAt: '2026-09-30T12:10:00Z',
      updatedAt: '2026-09-30T12:10:00Z',
    }]);
    mockGetProvenance.mockResolvedValue({
      id: 'prov_candidate_1',
      sourceType: 'session-event',
      sourceId: 'event_1',
      confidence: 0.94,
      observedAt: '2026-09-30T12:09:00Z',
      createdAt: '2026-09-30T12:10:00Z',
      evidence: [{
        source: 'file',
        path: 'docs/KNOWLEDGE_CORE.md',
        quote: 'Pending candidates do not enter retrieval.',
      }],
    });
    mockGetSession.mockResolvedValue({
      id: 'session_1',
      agentId: 'architect',
      project: 'agentvault',
      objective: 'Define memory governance',
      status: 'active',
      startedAt: '2026-09-30T10:00:00Z',
      updatedAt: '2026-09-30T12:00:00Z',
      events: [],
    });
    mockGetObject.mockResolvedValue({
      id: 'obj_1',
      type: 'project',
      title: 'AgentVault',
      project: 'agentvault',
      createdAt: '2026-09-29T10:00:00Z',
      updatedAt: '2026-09-30T10:00:00Z',
    });

    render(<ActivityView />);
    fireEvent.click(screen.getByRole('button', { name: 'Memory review' }));
    fireEvent.click(await screen.findByRole('button', { name: /Semantic memory requires explicit review/i }));

    expect(await screen.findByText('episode_candidate_1')).toBeInTheDocument();
    expect(screen.getByText('Pending candidates do not enter retrieval.')).toBeInTheDocument();
    expect(screen.getByText('Define memory governance')).toBeInTheDocument();
    expect(screen.getByText('AgentVault')).toBeInTheDocument();
    expect(mockGetProvenance).toHaveBeenCalledWith('prov_candidate_1');
    expect(mockGetSession).toHaveBeenCalledWith('session_1');
    expect(mockGetObject).toHaveBeenCalledWith('obj_1');
  });

  it('accepts a candidate only with an explicit reviewer and removes it from the pending queue', async () => {
    mockListMemoryCandidates.mockResolvedValue([{
      id: 'candidate_1',
      sourceEpisodeId: 'episode_candidate_1',
      memoryKind: 'decision',
      scopeType: 'project',
      scopeId: 'agentvault',
      content: 'Keep deterministic extraction reviewable.',
      provenanceId: 'prov_candidate_1',
      confidence: 0.91,
      status: 'pending',
      createdAt: '2026-09-30T12:10:00Z',
      updatedAt: '2026-09-30T12:10:00Z',
    }]);

    render(<ActivityView />);
    fireEvent.click(screen.getByRole('button', { name: 'Memory review' }));
    fireEvent.click(await screen.findByRole('button', { name: /Keep deterministic extraction reviewable/i }));

    const accept = screen.getByRole('button', { name: 'Accept memory' });
    expect(accept).toBeDisabled();

    fireEvent.change(screen.getByLabelText('Reviewer'), { target: { value: 'david' } });
    fireEvent.change(screen.getByLabelText('Review reason'), { target: { value: 'Verified against source evidence.' } });
    fireEvent.click(accept);

    await waitFor(() =>
      expect(mockAcceptMemoryCandidate).toHaveBeenCalledWith('candidate_1', {
        reviewedBy: 'david',
        reason: 'Verified against source evidence.',
      }),
    );
    expect(await screen.findByText('Candidate accepted.')).toBeInTheDocument();
    expect(screen.queryByText('Keep deterministic extraction reviewable.')).not.toBeInTheDocument();
  });

  it('rejects a candidate without creating durable memory', async () => {
    mockListMemoryCandidates.mockResolvedValue([{
      id: 'candidate_1',
      sourceEpisodeId: 'episode_candidate_1',
      memoryKind: 'fact',
      scopeType: 'project',
      scopeId: 'agentvault',
      content: 'Unverified semantic claim.',
      provenanceId: 'prov_candidate_1',
      confidence: 0.55,
      status: 'pending',
      createdAt: '2026-09-30T12:10:00Z',
      updatedAt: '2026-09-30T12:10:00Z',
    }]);

    render(<ActivityView />);
    fireEvent.click(screen.getByRole('button', { name: 'Memory review' }));
    fireEvent.click(await screen.findByRole('button', { name: /Unverified semantic claim/i }));
    fireEvent.change(screen.getByLabelText('Reviewer'), { target: { value: 'reviewer' } });
    fireEvent.click(screen.getByRole('button', { name: 'Reject candidate' }));

    await waitFor(() =>
      expect(mockRejectMemoryCandidate).toHaveBeenCalledWith('candidate_1', {
        reviewedBy: 'reviewer',
        reason: undefined,
      }),
    );
    expect(await screen.findByText('Candidate rejected.')).toBeInTheDocument();
  });

});
