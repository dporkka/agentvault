import AsyncStorage from '@react-native-async-storage/async-storage';
import {
  addCapture,
  getCaptures,
  getUnsyncedCaptures,
  updateCapture,
  markAsSynced,
  markAsSyncing,
  markAsFailed,
  setSyncStatus,
  deleteCapture,
  getSettings,
  saveSettings,
  clearInbox,
} from '../localInbox';
import { DEFAULT_APP_SETTINGS } from '../settingsStore';
import type { Capture } from '../../types';

jest.mock('../settingsStore', () => ({
  DEFAULT_APP_SETTINGS: {
    serverUrl: 'http://127.0.0.1:47321',
    defaultProject: '',
    token: '',
  },
  loadSettings: jest.fn(),
  persistSettings: jest.fn(),
}));

const { loadSettings, persistSettings } = jest.requireMock('../settingsStore');

describe('addCapture', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
  });

  it('adds a capture and returns it with generated metadata', async () => {
    const capture = await addCapture({
      type: 'text',
      title: 'Hello',
      text: 'body',
      project: 'inbox',
      tags: ['idea'],
    });

    expect(capture.id).toBeDefined();
    expect(capture.createdAt).toBeDefined();
    expect(capture.synced).toBe(false);
    expect(capture.syncStatus).toBe('unsynced');
    expect(capture.retryCount).toBe(0);
    expect(AsyncStorage.setItem).toHaveBeenCalled();
  });
});

describe('getCaptures', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('returns an empty array when nothing is stored', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
    const result = await getCaptures();
    expect(result).toEqual([]);
  });

  it('parses and returns stored captures', async () => {
    const stored: Capture[] = [
      {
        id: '1',
        type: 'text',
        title: 'A',
        tags: [],
        createdAt: new Date().toISOString(),
        synced: false,
      },
    ];
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(JSON.stringify(stored));
    const result = await getCaptures();
    expect(result).toHaveLength(1);
    expect(result[0].syncStatus).toBe('unsynced');
  });

  it('resets to an empty array when stored data is not an array', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue('{"invalid":true}');
    const result = await getCaptures();
    expect(result).toEqual([]);
  });

  it('resets to an empty array when stored data is invalid JSON', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue('not json');
    const result = await getCaptures();
    expect(result).toEqual([]);
  });
});

describe('getUnsyncedCaptures', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('only returns captures that are not synced and not syncing', async () => {
    const stored: Capture[] = [
      {
        id: '1',
        type: 'text',
        title: 'A',
        tags: [],
        createdAt: '',
        synced: false,
        syncStatus: 'unsynced',
      },
      {
        id: '2',
        type: 'text',
        title: 'B',
        tags: [],
        createdAt: '',
        synced: true,
        syncStatus: 'synced',
      },
      {
        id: '3',
        type: 'text',
        title: 'C',
        tags: [],
        createdAt: '',
        synced: false,
        syncStatus: 'syncing',
      },
    ];
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(JSON.stringify(stored));

    const result = await getUnsyncedCaptures();
    expect(result).toHaveLength(1);
    expect(result[0].id).toBe('1');
  });
});

describe('updateCapture', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('applies the patch to the matching capture', async () => {
    const stored: Capture[] = [
      { id: '1', type: 'text', title: 'A', tags: [], createdAt: '', synced: false },
    ];
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(JSON.stringify(stored));

    await updateCapture('1', { title: 'Updated' });

    const saved = JSON.parse((AsyncStorage.setItem as jest.Mock).mock.calls[0][1]);
    expect(saved[0].title).toBe('Updated');
  });
});

describe('markAsSynced', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('marks the capture as synced and clears errors', async () => {
    const stored: Capture[] = [
      {
        id: '1',
        type: 'text',
        title: 'A',
        tags: [],
        createdAt: '',
        synced: false,
        syncStatus: 'unsynced',
        syncError: 'old',
      },
    ];
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(JSON.stringify(stored));

    await markAsSynced('1');

    const saved = JSON.parse((AsyncStorage.setItem as jest.Mock).mock.calls[0][1]);
    expect(saved[0]).toMatchObject({ synced: true, syncStatus: 'synced' });
    expect(saved[0]).not.toHaveProperty('syncError');
  });
});

describe('markAsSyncing', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('marks the capture as syncing and clears errors', async () => {
    const stored: Capture[] = [
      {
        id: '1',
        type: 'text',
        title: 'A',
        tags: [],
        createdAt: '',
        synced: false,
        syncError: 'old',
      },
    ];
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(JSON.stringify(stored));

    await markAsSyncing('1');

    const saved = JSON.parse((AsyncStorage.setItem as jest.Mock).mock.calls[0][1]);
    expect(saved[0]).toMatchObject({ syncStatus: 'syncing' });
    expect(saved[0]).not.toHaveProperty('syncError');
  });
});

describe('markAsFailed', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('marks the capture as failed and increments retry count', async () => {
    const stored: Capture[] = [
      {
        id: '1',
        type: 'text',
        title: 'A',
        tags: [],
        createdAt: '',
        synced: false,
        syncStatus: 'syncing',
        retryCount: 2,
      },
    ];
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(JSON.stringify(stored));

    await markAsFailed('1', 'failed again');

    const saved = JSON.parse((AsyncStorage.setItem as jest.Mock).mock.calls[0][1]);
    expect(saved[0]).toMatchObject({
      synced: false,
      syncStatus: 'failed',
      syncError: 'failed again',
      retryCount: 3,
    });
    expect(saved[0].lastRetryAt).toBeDefined();
  });
});

describe('setSyncStatus', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('updates only the sync status', async () => {
    const stored: Capture[] = [
      { id: '1', type: 'text', title: 'A', tags: [], createdAt: '', synced: false },
    ];
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(JSON.stringify(stored));

    await setSyncStatus('1', 'failed');

    const saved = JSON.parse((AsyncStorage.setItem as jest.Mock).mock.calls[0][1]);
    expect(saved[0].syncStatus).toBe('failed');
  });
});

describe('deleteCapture', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('removes the capture by id', async () => {
    const stored: Capture[] = [
      { id: '1', type: 'text', title: 'A', tags: [], createdAt: '', synced: false },
      { id: '2', type: 'text', title: 'B', tags: [], createdAt: '', synced: false },
    ];
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(JSON.stringify(stored));

    await deleteCapture('1');

    const saved = JSON.parse((AsyncStorage.setItem as jest.Mock).mock.calls[0][1]);
    expect(saved).toHaveLength(1);
    expect(saved[0].id).toBe('2');
  });
});

describe('getSettings', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('returns loaded settings', async () => {
    const settings = { ...DEFAULT_APP_SETTINGS, token: 'secret' };
    loadSettings.mockResolvedValue(settings);

    const result = await getSettings();
    expect(result).toEqual(settings);
  });

  it('falls back to defaults when loading fails', async () => {
    loadSettings.mockRejectedValue(new Error('locked'));

    const result = await getSettings();
    expect(result).toEqual(DEFAULT_APP_SETTINGS);
  });
});

describe('saveSettings', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('persists settings through the settings store', async () => {
    const settings = { ...DEFAULT_APP_SETTINGS, defaultProject: 'work' };
    persistSettings.mockResolvedValue(undefined);

    await saveSettings(settings);
    expect(persistSettings).toHaveBeenCalledWith(settings);
  });
});

describe('clearInbox', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('removes the inbox key', async () => {
    await clearInbox();
    expect(AsyncStorage.removeItem).toHaveBeenCalledWith('agentvault_inbox');
  });
});
