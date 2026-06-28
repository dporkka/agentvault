import { updateClientConfig, DEFAULT_BASE_URL } from '../agentvault';

describe('updateClientConfig', () => {
  it('updates the base URL by removing a trailing slash', () => {
    expect(() => updateClientConfig('http://example.com:47321/', undefined)).not.toThrow();
  });

  it('accepts the default base URL', () => {
    expect(() => updateClientConfig(DEFAULT_BASE_URL, undefined)).not.toThrow();
  });

  it('updates the auth token', () => {
    expect(() => updateClientConfig(undefined, 'test-token')).not.toThrow();
  });
});
