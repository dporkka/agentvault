import AsyncStorage from '@react-native-async-storage/async-storage';
import * as SecureStore from 'expo-secure-store';
import { DEFAULT_BASE_URL } from '@agentvault/contract';
import type { AppSettings } from '../types';

const SETTINGS_KEY = 'agentvault_settings';
const TOKEN_KEY = 'agentvault_token';

export const DEFAULT_APP_SETTINGS: AppSettings = {
  serverUrl: DEFAULT_BASE_URL,
  defaultProject: '',
  token: '',
};

/**
 * Load settings. Non-sensitive settings come from AsyncStorage; the auth token
 * is read from SecureStore only. If SecureStore cannot be accessed, the token
 * is treated as missing and the user must re-authenticate.
 */
export async function loadSettings(): Promise<AppSettings> {
  const [data, secureToken] = await Promise.all([
    AsyncStorage.getItem(SETTINGS_KEY),
    SecureStore.getItemAsync(TOKEN_KEY).catch(() => null),
  ]);

  let parsed: Partial<AppSettings> = {};
  if (data) {
    try {
      parsed = JSON.parse(data);
    } catch {
      parsed = {};
    }
  }

  return {
    ...DEFAULT_APP_SETTINGS,
    ...parsed,
    token: secureToken ?? '',
  };
}

/**
 * Persist settings. Non-sensitive settings go to AsyncStorage; the auth token
 * is stored only in SecureStore. If SecureStore fails, the error propagates so
 * the UI can warn the user instead of silently falling back to insecure storage.
 */
export async function persistSettings(settings: AppSettings): Promise<void> {
  const { token, ...rest } = settings;
  await AsyncStorage.setItem(SETTINGS_KEY, JSON.stringify(rest));

  if (token) {
    await SecureStore.setItemAsync(TOKEN_KEY, token);
  } else {
    // Ignore deletion failures; there is no secret to leak when clearing the token.
    await SecureStore.deleteItemAsync(TOKEN_KEY).catch(() => {});
  }
}
