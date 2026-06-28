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
 * Load settings. The auth token is stored in SecureStore; everything else
 * is stored in AsyncStorage. If a token exists in AsyncStorage from a
 * previous version, it is migrated to SecureStore.
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

  // Migrate token from AsyncStorage to SecureStore if needed.
  if (!secureToken && parsed.token) {
    await SecureStore.setItemAsync(TOKEN_KEY, parsed.token).catch(() => {
      // If SecureStore fails, fall back to keeping it in AsyncStorage.
    });
  }

  return {
    ...DEFAULT_APP_SETTINGS,
    ...parsed,
    token: secureToken ?? parsed.token ?? '',
  };
}

/**
 * Persist settings. The auth token is written to SecureStore separately.
 */
export async function persistSettings(settings: AppSettings): Promise<void> {
  const { token, ...rest } = settings;
  await Promise.all([
    AsyncStorage.setItem(SETTINGS_KEY, JSON.stringify(rest)),
    token
      ? SecureStore.setItemAsync(TOKEN_KEY, token).catch(() => {
          // If SecureStore is unavailable, keep the token in AsyncStorage.
          AsyncStorage.setItem(SETTINGS_KEY, JSON.stringify({ ...rest, token }));
        })
      : SecureStore.deleteItemAsync(TOKEN_KEY).catch(() => {
          AsyncStorage.setItem(SETTINGS_KEY, JSON.stringify(rest));
        }),
  ]);
}
