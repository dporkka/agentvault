import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

const createClient = vi.fn();
const getDefaultClient = vi.fn();

vi.mock('@agentvault/contract', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@agentvault/contract')>();
  return {
    ...actual,
    createClient,
    getDefaultClient,
  };
});

describe('api client singleton', () => {
  beforeEach(() => {
    createClient.mockReset();
    getDefaultClient.mockReset();
  });

  afterEach(() => {
    vi.resetModules();
  });

  it('uses the default localStorage-backed client in the browser', async () => {
    const browserClient = { id: 'browser' };
    getDefaultClient.mockReturnValue(browserClient);

    vi.stubGlobal('window', { localStorage: {} as Storage });
    const { api } = await import('../client');

    expect(getDefaultClient).toHaveBeenCalled();
    expect(api).toBe(browserClient);

    vi.unstubAllGlobals();
  });

  it('falls back to a fresh client when window is unavailable', async () => {
    const ssrClient = { id: 'ssr' };
    createClient.mockReturnValue(ssrClient);

    vi.stubGlobal('window', undefined);
    const { api } = await import('../client');

    expect(createClient).toHaveBeenCalled();
    expect(api).toBe(ssrClient);

    vi.unstubAllGlobals();
  });
});
