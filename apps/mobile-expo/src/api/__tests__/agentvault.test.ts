import {
  createMobileClient,
  DEFAULT_BASE_URL,
  checkHealth,
  sendCapture,
  searchVault,
  getProjects,
  getNote,
  getRecentNotes,
  verifyToken,
} from '../agentvault';
import { getSettings } from '../../storage/localInbox';
import { createClient } from '@agentvault/contract';
import type { Capture } from '../../types';

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

describe('sendCapture', () => {
  const capturePayload: Omit<Capture, 'id' | 'synced' | 'createdAt'> = {
    type: 'text',
    title: 'Test capture',
    text: 'body',
    project: 'work',
    tags: ['idea'],
  };

  beforeEach(() => {
    jest.clearAllMocks();
    (getSettings as jest.Mock).mockResolvedValue({
      serverUrl: DEFAULT_BASE_URL,
      token: '',
      defaultProject: '',
    });
  });

  it('sends the capture payload through the client', async () => {
    const capture = jest.fn().mockResolvedValue(undefined);
    (createClient as jest.Mock).mockReturnValue({ capture });

    await sendCapture(capturePayload);

    expect(capture).toHaveBeenCalledWith({
      type: 'text',
      title: 'Test capture',
      text: 'body',
      project: 'work',
      tags: ['idea'],
    });
  });

  it('throws a classified error when the client fails', async () => {
    (createClient as jest.Mock).mockReturnValue({
      capture: jest.fn().mockRejectedValue(new Error('network down')),
    });

    await expect(sendCapture(capturePayload)).rejects.toThrow('network down');
  });
});

describe('searchVault', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getSettings as jest.Mock).mockResolvedValue({
      serverUrl: DEFAULT_BASE_URL,
      token: '',
      defaultProject: '',
    });
  });

  it('returns an empty array for an empty string query', async () => {
    const result = await searchVault('   ');
    expect(result).toEqual([]);
    expect(createClient).not.toHaveBeenCalled();
  });

  it('searches with a string query', async () => {
    const search = jest.fn().mockResolvedValue([{ id: '1', title: 'Note' }]);
    (createClient as jest.Mock).mockReturnValue({ search });

    const result = await searchVault('hello');
    expect(search).toHaveBeenCalledWith({ q: 'hello' });
    expect(result).toEqual([{ id: '1', title: 'Note' }]);
  });

  it('searches with structured params and a custom URL', async () => {
    const search = jest.fn().mockResolvedValue([]);
    (createClient as jest.Mock).mockReturnValue({ search });

    await searchVault({ q: 'query', project: 'work', limit: 10 }, 'http://other.com:8080/');
    expect(createClient).toHaveBeenCalledWith({
      baseUrl: 'http://other.com:8080',
      token: '',
    });
    expect(search).toHaveBeenCalledWith({ q: 'query', project: 'work', limit: 10 });
  });
});

describe('getProjects', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getSettings as jest.Mock).mockResolvedValue({
      serverUrl: DEFAULT_BASE_URL,
      token: '',
      defaultProject: '',
    });
  });

  it('returns the project list from the client', async () => {
    (createClient as jest.Mock).mockReturnValue({
      getProjects: jest.fn().mockResolvedValue(['personal', 'work']),
    });

    const result = await getProjects();
    expect(result).toEqual(['personal', 'work']);
  });

  it('uses the provided URL override', async () => {
    (createClient as jest.Mock).mockReturnValue({
      getProjects: jest.fn().mockResolvedValue([]),
    });

    await getProjects('http://custom.com:8080/');
    expect(createClient).toHaveBeenCalledWith({
      baseUrl: 'http://custom.com:8080',
      token: '',
    });
  });
});

describe('getNote', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getSettings as jest.Mock).mockResolvedValue({
      serverUrl: DEFAULT_BASE_URL,
      token: '',
      defaultProject: '',
    });
  });

  it('returns the note detail from the client', async () => {
    const note = { id: 'note-1', title: 'Note', content: 'body' };
    (createClient as jest.Mock).mockReturnValue({
      getNote: jest.fn().mockResolvedValue(note),
    });

    await expect(getNote('note-1')).resolves.toEqual(note);
  });
});

describe('getRecentNotes', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getSettings as jest.Mock).mockResolvedValue({
      serverUrl: DEFAULT_BASE_URL,
      token: '',
      defaultProject: '',
    });
  });

  it('returns recent notes with an optional limit', async () => {
    const getRecent = jest.fn().mockResolvedValue([{ id: '1', title: 'Recent' }]);
    (createClient as jest.Mock).mockReturnValue({ getRecent });

    const result = await getRecentNotes(5);
    expect(getRecent).toHaveBeenCalledWith({ limit: 5 });
    expect(result).toEqual([{ id: '1', title: 'Recent' }]);
  });
});

describe('verifyToken', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (getSettings as jest.Mock).mockResolvedValue({
      serverUrl: DEFAULT_BASE_URL,
      token: '',
      defaultProject: '',
    });
  });

  it('returns the auth verification response', async () => {
    const response = { valid: true, user: { id: 'u1' } };
    (createClient as jest.Mock).mockReturnValue({
      verifyAuth: jest.fn().mockResolvedValue(response),
    });

    await expect(verifyToken()).resolves.toEqual(response);
  });

  it('classifies errors thrown by the client', async () => {
    (createClient as jest.Mock).mockReturnValue({
      verifyAuth: jest.fn().mockRejectedValue(new Error('auth failed')),
    });

    await expect(verifyToken()).rejects.toThrow('auth failed');
  });
});
