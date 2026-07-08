import { describe, it, expect, vi, beforeEach } from 'vitest';
import { installChromeStorageMock } from './storage-mock';
import { ApiError } from '@agentvault/contract';

const mockClient = vi.hoisted(() => ({
  setToken: vi.fn(),
  setBaseUrl: vi.fn(),
  getBaseUrl: vi.fn().mockReturnValue('http://localhost:47321'),
  checkHealth: vi.fn(),
  verifyAuth: vi.fn(),
  getVaultStatus: vi.fn(),
  getProjects: vi.fn(),
  getRecent: vi.fn(),
  getStale: vi.fn(),
  search: vi.fn(),
  getNote: vi.fn(),
  capture: vi.fn(),
  createNote: vi.fn(),
  ask: vi.fn(),
  triggerIndex: vi.fn(),
  getGitStatus: vi.fn(),
}));

vi.mock('@agentvault/contract', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@agentvault/contract')>();
  return {
    ...actual,
    createClient: vi.fn(() => mockClient),
  };
});

import {
  getToken,
  setToken,
  getBaseUrl,
  setBaseUrl,
  syncClientConfig,
  checkHealth,
  checkAuth,
  getVaultStatus,
  getProjects,
  getRecent,
  searchVault,
  sendCapture,
  refreshToken,
  refreshBaseUrl,
  getNote,
  getNoteDetail,
  createNote,
  ask,
  askVault,
  getRecentNotes,
  getStale,
  getGitStatus,
  triggerIndex,
} from '../api';

describe('api', () => {
  beforeEach(() => {
    installChromeStorageMock();
    for (const key of Object.keys(mockClient)) {
      const fn = mockClient[key as keyof typeof mockClient];
      if (typeof fn === 'function' && 'mockReset' in fn) {
        (fn as ReturnType<typeof vi.fn>).mockReset();
      }
    }
    mockClient.getBaseUrl.mockReturnValue('http://localhost:47321');
  });

  describe('token and base URL storage', () => {
    it('reads the stored token', async () => {
      const { storage } = installChromeStorageMock();
      storage.local.set({ agentvault_token: 'abc' });
      expect(await getToken()).toBe('abc');
    });

    it('defaults to empty token when none is stored', async () => {
      expect(await getToken()).toBe('');
    });

    it('stores the token and updates the client', async () => {
      await setToken('xyz');
      expect(await getToken()).toBe('xyz');
      expect(mockClient.setToken).toHaveBeenCalledWith('xyz');
    });

    it('reads the stored base URL', async () => {
      const { storage } = installChromeStorageMock();
      storage.local.set({ agentvault_base_url: 'http://server:8080' });
      expect(await getBaseUrl()).toBe('http://localhost:47321');
      expect(mockClient.setBaseUrl).toHaveBeenCalledWith('http://server:8080');
    });

    it('stores a normalized base URL', async () => {
      await setBaseUrl('http://server:8080/');
      expect(mockClient.setBaseUrl).toHaveBeenCalledWith('http://server:8080');
    });
  });

  describe('syncClientConfig', () => {
    it('loads token and base URL from storage into the client', async () => {
      const { storage } = installChromeStorageMock();
      storage.local.set({ agentvault_token: 'tok', agentvault_base_url: 'http://srv:9000' });
      await syncClientConfig();
      expect(mockClient.setToken).toHaveBeenCalledWith('tok');
      expect(mockClient.setBaseUrl).toHaveBeenCalledWith('http://srv:9000');
    });
  });

  describe('checkHealth', () => {
    it('returns true when the server is healthy', async () => {
      mockClient.checkHealth.mockResolvedValue({ ok: true });
      expect(await checkHealth()).toBe(true);
    });

    it('returns false when the server is unreachable', async () => {
      mockClient.checkHealth.mockRejectedValue(new Error('network'));
      expect(await checkHealth()).toBe(false);
    });
  });

  describe('checkAuth', () => {
    it('returns the verify response on success', async () => {
      const response = { hasToken: true, tokenValid: true };
      mockClient.verifyAuth.mockResolvedValue(response);
      expect(await checkAuth()).toEqual(response);
    });

    it('returns null on error', async () => {
      mockClient.verifyAuth.mockRejectedValue(new Error('nope'));
      expect(await checkAuth()).toBeNull();
    });
  });

  describe('getVaultStatus', () => {
    it('returns the vault status on success', async () => {
      const status = { isVault: true, noteCount: 5 };
      mockClient.getVaultStatus.mockResolvedValue(status);
      expect(await getVaultStatus()).toEqual(status);
    });

    it('returns null on error', async () => {
      mockClient.getVaultStatus.mockRejectedValue(new Error('nope'));
      expect(await getVaultStatus()).toBeNull();
    });
  });

  describe('getProjects', () => {
    it('returns the project list on success', async () => {
      mockClient.getProjects.mockResolvedValue(['work', 'personal']);
      expect(await getProjects()).toEqual(['work', 'personal']);
    });

    it('returns an empty list on error', async () => {
      mockClient.getProjects.mockRejectedValue(new Error('nope'));
      expect(await getProjects()).toEqual([]);
    });
  });

  describe('getRecent', () => {
    it('returns recent notes', async () => {
      const notes = [{ id: '1', title: 'A' }];
      mockClient.getRecent.mockResolvedValue(notes);
      expect(await getRecent({ limit: 5 })).toEqual(notes);
    });
  });

  describe('searchVault', () => {
    it('returns empty results for an empty query', async () => {
      expect(await searchVault('')).toEqual([]);
      expect(await searchVault({ q: '   ' })).toEqual([]);
      expect(mockClient.search).not.toHaveBeenCalled();
    });

    it('searches with a string query', async () => {
      const notes = [{ id: '1', title: 'A' }];
      mockClient.search.mockResolvedValue(notes);
      expect(await searchVault('hello')).toEqual(notes);
      expect(mockClient.search).toHaveBeenCalledWith({ q: 'hello' });
    });

    it('passes filters through', async () => {
      await searchVault({ q: 'hello', type: 'note' });
      expect(mockClient.search).toHaveBeenCalledWith({ q: 'hello', type: 'note' });
    });
  });

  describe('sendCapture', () => {
    it('strips client-only fields before sending', async () => {
      mockClient.capture.mockResolvedValue({ path: 'notes/x.md' });
      const result = await sendCapture({
        type: 'selection',
        title: 'T',
        url: 'http://example.com',
        text: 'body',
        selectedText: 'body',
        project: 'work',
        tags: ['a'],
        capturedAt: new Date().toISOString(),
      });
      expect(mockClient.capture).toHaveBeenCalledWith({
        type: 'selection',
        title: 'T',
        url: 'http://example.com',
        text: 'body',
        project: 'work',
        tags: ['a'],
      });
      expect(result.path).toBe('notes/x.md');
    });

    it('falls back to selectedText when text is missing', async () => {
      mockClient.capture.mockResolvedValue({ path: 'notes/y.md' });
      await sendCapture({
        type: 'selection',
        title: 'T',
        url: 'http://example.com',
        selectedText: 'selection text',
        capturedAt: new Date().toISOString(),
      });
      expect(mockClient.capture).toHaveBeenCalledWith(
        expect.objectContaining({ text: 'selection text' }),
      );
    });

    it('surfaces ApiErrors', async () => {
      mockClient.capture.mockRejectedValue(new ApiError('Bad request', 400));
      await expect(
        sendCapture({
          type: 'webpage',
          title: 'T',
          url: 'http://example.com',
          capturedAt: new Date().toISOString(),
        }),
      ).rejects.toThrow('Bad request');
    });
  });

  describe('refreshToken', () => {
    it('reads the stored token into the client', async () => {
      const { storage } = installChromeStorageMock();
      storage.local.set({ agentvault_token: 'refreshed' });
      await refreshToken();
      expect(mockClient.setToken).toHaveBeenCalledWith('refreshed');
    });
  });

  describe('refreshBaseUrl', () => {
    it('reads the stored base URL into the client', async () => {
      const { storage } = installChromeStorageMock();
      storage.local.set({ agentvault_base_url: 'http://custom:9000' });
      await refreshBaseUrl();
      expect(mockClient.setBaseUrl).toHaveBeenCalledWith('http://custom:9000');
    });

    it('falls back to the default base URL when none is stored', async () => {
      await refreshBaseUrl();
      expect(mockClient.setBaseUrl).toHaveBeenCalledWith('http://127.0.0.1:47321');
    });
  });

  describe('getNote / getNoteDetail', () => {
    it('returns the note on success', async () => {
      const note = { id: 'n1', title: 'Note', content: 'body', path: 'notes/n1.md', tags: [] };
      mockClient.getNote.mockResolvedValue(note);
      expect(await getNote('n1')).toEqual(note);
      expect(await getNoteDetail('n1')).toEqual(note);
      expect(mockClient.getNote).toHaveBeenCalledWith('n1');
    });

    it('returns null on error', async () => {
      mockClient.getNote.mockRejectedValue(new Error('nope'));
      expect(await getNote('n1')).toBeNull();
    });
  });

  describe('createNote', () => {
    it('passes the request through to the client', async () => {
      const response = { path: 'notes/new.md' };
      mockClient.createNote.mockResolvedValue(response);
      const req = { title: 'New', type: 'note', tags: ['a'] };
      expect(await createNote(req)).toEqual(response);
      expect(mockClient.createNote).toHaveBeenCalledWith(req);
    });

    it('surfaces errors', async () => {
      mockClient.createNote.mockRejectedValue(new Error('nope'));
      await expect(createNote({ title: 'New', type: 'note' })).rejects.toThrow('nope');
    });
  });

  describe('ask / askVault', () => {
    it('asks the vault with a question string alias', async () => {
      const response = { answer: 'yes', sources: [] };
      mockClient.ask.mockResolvedValue(response);
      expect(await askVault('why?')).toEqual(response);
      expect(mockClient.ask).toHaveBeenCalledWith({ question: 'why?' });
    });

    it('accepts a full ask request', async () => {
      const response = { answer: 'maybe', sources: [] };
      mockClient.ask.mockResolvedValue(response);
      expect(await ask({ question: 'how?' })).toEqual(response);
    });

    it('surfaces errors', async () => {
      mockClient.ask.mockRejectedValue(new Error('nope'));
      await expect(ask({ question: '?' })).rejects.toThrow('nope');
    });
  });

  describe('getRecentNotes', () => {
    it('is an alias for getRecent', async () => {
      const notes = [{ id: '1', title: 'A' }];
      mockClient.getRecent.mockResolvedValue(notes);
      expect(await getRecentNotes({ limit: 3 })).toEqual(notes);
      expect(mockClient.getRecent).toHaveBeenCalledWith({ limit: 3 });
    });
  });

  describe('getStale', () => {
    it('returns stale notes on success', async () => {
      const notes = [{ id: '1', title: 'Stale' }];
      mockClient.getStale.mockResolvedValue(notes);
      expect(await getStale({ days: 7 })).toEqual(notes);
      expect(mockClient.getStale).toHaveBeenCalledWith({ days: 7 });
    });

    it('returns an empty list on error', async () => {
      mockClient.getStale.mockRejectedValue(new Error('nope'));
      expect(await getStale()).toEqual([]);
    });
  });

  describe('getGitStatus', () => {
    it('returns the git status on success', async () => {
      const status = { branch: 'main', clean: true };
      mockClient.getGitStatus.mockResolvedValue(status);
      expect(await getGitStatus()).toEqual(status);
    });

    it('returns null on error', async () => {
      mockClient.getGitStatus.mockRejectedValue(new Error('nope'));
      expect(await getGitStatus()).toBeNull();
    });
  });

  describe('triggerIndex', () => {
    it('returns the index result on success', async () => {
      const result = { indexed: 5 };
      mockClient.triggerIndex.mockResolvedValue(result);
      expect(await triggerIndex({ force: true })).toEqual(result);
      expect(mockClient.triggerIndex).toHaveBeenCalledWith({ force: true });
    });

    it('returns null on error', async () => {
      mockClient.triggerIndex.mockRejectedValue(new Error('nope'));
      expect(await triggerIndex()).toBeNull();
    });
  });

  describe('setBaseUrl', () => {
    it('requests host permission for the new origin', async () => {
      const request = vi.fn().mockResolvedValue(true);
      (globalThis as unknown as Record<string, unknown>).chrome = {
        storage: installChromeStorageMock().storage,
        permissions: { request },
      } as unknown as typeof chrome;
      await setBaseUrl('http://remote.server:9000/');
      expect(request).toHaveBeenCalledWith({ origins: ['http://remote.server:9000/*'] });
      expect(mockClient.setBaseUrl).toHaveBeenCalledWith('http://remote.server:9000');
    });

    it('ignores missing permissions API', async () => {
      (globalThis as unknown as Record<string, unknown>).chrome = {
        storage: installChromeStorageMock().storage,
      } as unknown as typeof chrome;
      await expect(setBaseUrl('http://safe.example/')).resolves.toBeUndefined();
    });
  });
});
