import { describe, it, expect, vi } from 'vitest';
import { createClient } from '@agentvault/contract';

describe('context client', () => {
  it('posts context compilation requests to /context/compile', async () => {
    const fetchImpl = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        version: '1',
        task: 'test',
        asOf: '2026-09-30T12:00:00Z',
        tokenBudget: 8000,
        estimatedTokens: 10,
        truncated: false,
        items: [],
        stats: { candidates: 0, included: 0, dropped: 0, byKind: {} },
      }),
    });

    const client = createClient({
      baseUrl: 'http://127.0.0.1:47321',
      token: 'secret',
      fetchImpl: fetchImpl as unknown as typeof fetch,
    });

    const result = await client.compileContext({
      task: 'test',
      project: 'alpha',
      tokenBudget: 8000,
    });

    expect(result.task).toBe('test');
    expect(fetchImpl).toHaveBeenCalledWith(
      'http://127.0.0.1:47321/context/compile',
      expect.objectContaining({
        method: 'POST',
        headers: expect.objectContaining({
          'Content-Type': 'application/json',
          'X-AgentVault-Token': 'secret',
        }),
        body: JSON.stringify({
          task: 'test',
          project: 'alpha',
          tokenBudget: 8000,
        }),
      }),
    );
  });
});
