import { describe, it, expect } from 'vitest';
import { createClient, ApiError, inMemoryTokenStore, DEFAULT_BASE_URL } from './client';

function mockResponse(body: unknown, init: ResponseInit = {}): Response {
  return new Response(
    typeof body === 'string' ? body : JSON.stringify(body),
    {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
      ...init,
    }
  );
}

describe('ApiError', () => {
  it('carries the status code', () => {
    const err = new ApiError('unauthorized', 401);
    expect(err.status).toBe(401);
    expect(err.message).toBe('unauthorized');
    expect(err.name).toBe('ApiError');
  });
});

describe('createClient', () => {
  it('normalizes a trailing slash on baseUrl', () => {
    const client = createClient({ baseUrl: 'http://localhost:47321/' });
    expect(client.getBaseUrl()).toBe('http://localhost:47321');
  });

  it('uses the default base URL when none is provided', () => {
    const client = createClient();
    expect(client.getBaseUrl()).toBe(DEFAULT_BASE_URL);
  });

  it('stores and returns the token', () => {
    const client = createClient({ token: 'secret' });
    expect(client.getToken()).toBe('secret');
    client.setToken('new-secret');
    expect(client.getToken()).toBe('new-secret');
    client.setToken('');
    expect(client.getToken()).toBe('');
  });

  it('maps hybridWeight to hybrid_weight in search params', async () => {
    const requests: string[] = [];
    const client = createClient({
      fetchImpl: async (url) => {
        requests.push(url.toString());
        return mockResponse([]);
      },
    });
    await client.search({ q: 'test', hybridWeight: 0.5 });
    expect(requests[0]).toContain('hybrid_weight=0.5');
    expect(requests[0]).not.toContain('hybridWeight');
  });

  it('throws ApiError on network failure with status 0', async () => {
    const client = createClient({
      fetchImpl: async () => {
        throw new Error('fetch failed');
      },
    });
    await expect(client.checkHealth()).rejects.toBeInstanceOf(ApiError);
    await expect(client.checkHealth()).rejects.toMatchObject({ status: 0 });
  });

  it('throws ApiError on non-JSON success response', async () => {
    const client = createClient({
      fetchImpl: async () =>
        new Response('not json', { status: 200, headers: { 'Content-Type': 'text/plain' } }),
    });
    await expect(client.checkHealth()).rejects.toBeInstanceOf(ApiError);
    await expect(client.checkHealth()).rejects.toMatchObject({ status: 500 });
  });

  it('surfaces JSON error body on HTTP error responses', async () => {
    const client = createClient({
      fetchImpl: async () =>
        mockResponse({ error: 'something went wrong' }, { status: 500 }),
    });
    await expect(client.checkHealth()).rejects.toBeInstanceOf(ApiError);
    await expect(client.checkHealth()).rejects.toMatchObject({
      status: 500,
      message: 'something went wrong',
    });
  });

  it('surfaces plain text on non-JSON error responses', async () => {
    const client = createClient({
      fetchImpl: async () =>
        new Response('bad gateway', { status: 502 }),
    });
    await expect(client.checkHealth()).rejects.toBeInstanceOf(ApiError);
    await expect(client.checkHealth()).rejects.toMatchObject({
      status: 502,
      message: 'bad gateway',
    });
  });
});

describe('inMemoryTokenStore', () => {
  it('get/set/clear', () => {
    const store = inMemoryTokenStore('a');
    expect(store.get()).toBe('a');
    store.set('b');
    expect(store.get()).toBe('b');
    store.clear();
    expect(store.get()).toBe('');
  });
});
