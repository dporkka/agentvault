import { createMobileClient, DEFAULT_BASE_URL, checkHealth } from '../agentvault';
import { getSettings } from '../../storage/localInbox';
import { createClient } from '@agentvault/contract';

jest.mock('../../storage/localInbox', () => ({
  getSettings: jest.fn(),
}));

describe('createMobileClient', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getSettings as jest.Mock).mockResolvedValue({
      serverUrl: 'http://example.com:47321/',
      token: 'test-token',
      defaultProject: '',
    });
  });

  it('normalizes the persisted base URL and passes the token', async () => {
    await createMobileClient();
    expect(createClient).toHaveBeenCalledWith({
      baseUrl: 'http://example.com:47321',
      token: 'test-token',
    });
  });

  it('allows overriding the base URL and token', async () => {
    await createMobileClient({ baseUrl: 'http://other.com:8080/', token: 'other-token' });
    expect(createClient).toHaveBeenCalledWith({
      baseUrl: 'http://other.com:8080',
      token: 'other-token',
    });
  });

  it('falls back to the default base URL and empty token', async () => {
    (getSettings as jest.Mock).mockResolvedValue({
      serverUrl: '',
      token: '',
      defaultProject: '',
    });
    await createMobileClient();
    expect(createClient).toHaveBeenCalledWith({
      baseUrl: DEFAULT_BASE_URL,
      token: '',
    });
  });
});

describe('checkHealth', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getSettings as jest.Mock).mockResolvedValue({
      serverUrl: DEFAULT_BASE_URL,
      token: '',
      defaultProject: '',
    });
  });

  it('returns true when the client reports healthy', async () => {
    (createClient as jest.Mock).mockReturnValue({
      checkHealth: jest.fn().mockResolvedValue({ ok: true }),
    });
    await expect(checkHealth()).resolves.toBe(true);
  });

  it('throws when the client throws', async () => {
    (createClient as jest.Mock).mockReturnValue({
      checkHealth: jest.fn().mockRejectedValue(new Error('boom')),
    });
    await expect(checkHealth()).rejects.toThrow('boom');
  });
});
