import AsyncStorage from '@react-native-async-storage/async-storage';
import * as SecureStore from 'expo-secure-store';
import { loadSettings, persistSettings, DEFAULT_APP_SETTINGS } from '../settingsStore';

jest.mock('expo-secure-store', () => ({
  getItemAsync: jest.fn().mockResolvedValue(null),
  setItemAsync: jest.fn().mockResolvedValue(undefined),
  deleteItemAsync: jest.fn().mockResolvedValue(undefined),
}));

describe('loadSettings', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('returns defaults when no data is stored', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
    (SecureStore.getItemAsync as jest.Mock).mockResolvedValue(null);

    const result = await loadSettings();
    expect(result).toEqual(DEFAULT_APP_SETTINGS);
  });

  it('merges stored settings and prefers the secure token', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(
      JSON.stringify({ serverUrl: 'http://custom.com', defaultProject: 'work', token: 'leaked' }),
    );
    (SecureStore.getItemAsync as jest.Mock).mockResolvedValue('secure-token');

    const result = await loadSettings();
    expect(result).toEqual({
      serverUrl: 'http://custom.com',
      defaultProject: 'work',
      token: 'secure-token',
    });
  });

  it('ignores invalid JSON in AsyncStorage', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue('not json');
    (SecureStore.getItemAsync as jest.Mock).mockResolvedValue(null);

    const result = await loadSettings();
    expect(result).toEqual(DEFAULT_APP_SETTINGS);
  });

  it('falls back to an empty token when SecureStore fails', async () => {
    (AsyncStorage.getItem as jest.Mock).mockResolvedValue(null);
    (SecureStore.getItemAsync as jest.Mock).mockRejectedValue(new Error('not available'));

    const result = await loadSettings();
    expect(result.token).toBe('');
  });
});

describe('persistSettings', () => {
  beforeEach(() => {
    jest.clearAllMocks();
  });

  it('writes non-sensitive settings to AsyncStorage and token to SecureStore', async () => {
    const settings = { serverUrl: 'http://custom.com', defaultProject: 'work', token: 'secret' };

    await persistSettings(settings);

    expect(AsyncStorage.setItem).toHaveBeenCalledWith(
      'agentvault_settings',
      JSON.stringify({ serverUrl: 'http://custom.com', defaultProject: 'work' }),
    );
    expect(SecureStore.setItemAsync).toHaveBeenCalledWith('agentvault_token', 'secret');
  });

  it('deletes the secure token when the token is empty', async () => {
    const settings = { serverUrl: 'http://custom.com', defaultProject: 'work', token: '' };

    await persistSettings(settings);

    expect(SecureStore.deleteItemAsync).toHaveBeenCalledWith('agentvault_token');
  });

  it('does not throw when token deletion fails', async () => {
    const settings = { serverUrl: 'http://custom.com', defaultProject: 'work', token: '' };
    (SecureStore.deleteItemAsync as jest.Mock).mockRejectedValue(new Error('not available'));

    await expect(persistSettings(settings)).resolves.toBeUndefined();
  });

  it('propagates errors when saving the token fails', async () => {
    const settings = { serverUrl: 'http://custom.com', defaultProject: 'work', token: 'secret' };
    (SecureStore.setItemAsync as jest.Mock).mockRejectedValue(new Error('locked'));

    await expect(persistSettings(settings)).rejects.toThrow('locked');
  });
});
