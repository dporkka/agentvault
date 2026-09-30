import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import ContextInspector from '../ContextInspector';

const mockCompileContext = vi.hoisted(() => vi.fn());
const mockGetProjects = vi.hoisted(() => vi.fn());

vi.mock('@/api/client', () => ({
  api: {
    compileContext: mockCompileContext,
    getProjects: mockGetProjects,
  },
}));

describe('ContextInspector', () => {
  beforeEach(() => {
    mockCompileContext.mockReset();
    mockGetProjects.mockReset();
    mockGetProjects.mockResolvedValue(['alpha']);
  });

  it('compiles context and exposes ranking, provenance, and token usage', async () => {
    const user = userEvent.setup();
    mockCompileContext.mockResolvedValue({
      version: '1',
      task: 'Ship context inspector',
      project: 'alpha',
      asOf: '2026-09-30T12:00:00Z',
      tokenBudget: 1000,
      estimatedTokens: 180,
      truncated: false,
      stats: {
        candidates: 4,
        included: 2,
        dropped: 2,
        byKind: { memory: 1, note: 1 },
      },
      items: [
        {
          kind: 'memory',
          id: 'mem-1',
          title: 'semantic decision memory',
          content: 'Use deterministic context compilation.',
          score: 0.96,
          estimatedTokens: 80,
          objectIds: ['obj-1'],
          provenance: {
            id: 'prov-1',
            sourceType: 'human',
            sourceId: 'note-1',
            confidence: 0.92,
            observedAt: '2026-09-29T10:00:00Z',
          },
          metadata: {
            memoryClass: 'semantic',
            memoryKind: 'decision',
          },
          ranking: {
            algorithm: 'deterministic-multisignal-v1',
            components: [
              { signal: 'sourcePrior', value: 0.9, weight: 0.64, contribution: 0.576 },
              { signal: 'lexicalRelevance', value: 1, weight: 0.16, contribution: 0.16 },
            ],
          },
        },
        {
          kind: 'note',
          id: 'note-1',
          title: 'Context design',
          content: 'Context compiler notes',
          path: '10-notes/context-design.md',
          score: 0.84,
          estimatedTokens: 100,
          objectIds: [],
          metadata: {},
        },
      ],
    });

    render(
      <MemoryRouter>
        <ContextInspector />
      </MemoryRouter>,
    );

    await user.type(screen.getByLabelText('Task'), 'Ship context inspector');
    await user.selectOptions(screen.getByLabelText('Project'), 'alpha');
    await user.click(screen.getByRole('button', { name: 'Compile context' }));

    await waitFor(() =>
      expect(mockCompileContext).toHaveBeenCalledWith(
        expect.objectContaining({
          task: 'Ship context inspector',
          project: 'alpha',
          tokenBudget: 8000,
          explain: true,
        }),
      ),
    );

    expect(screen.getByText('180 / 1,000 tokens')).toBeInTheDocument();
    expect(screen.getByText('2 included')).toBeInTheDocument();
    expect(screen.getByText('semantic decision memory')).toBeInTheDocument();
    expect(screen.getByText('Score 0.960')).toBeInTheDocument();
    expect(screen.getByText('human · 92% confidence')).toBeInTheDocument();
    expect(screen.getByText('Use deterministic context compilation.')).toBeInTheDocument();
    expect(screen.getByText('Why this ranked here')).toBeInTheDocument();
    expect(screen.getByText(/sourcePrior: 0\.90 × 0\.64 = 0\.576/)).toBeInTheDocument();
  });

  it('does not compile an empty task', async () => {
    const user = userEvent.setup();

    render(
      <MemoryRouter>
        <ContextInspector />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole('button', { name: 'Compile context' }));
    expect(mockCompileContext).not.toHaveBeenCalled();
  });
});
