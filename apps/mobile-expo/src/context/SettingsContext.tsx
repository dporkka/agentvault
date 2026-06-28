import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';
import type { AppSettings } from '../types';
import { updateClientConfig } from '../api/agentvault';
import { DEFAULT_APP_SETTINGS, loadSettings, persistSettings } from '../storage/settingsStore';

export { DEFAULT_APP_SETTINGS as DEFAULT_SETTINGS };
import AsyncStorage from '@react-native-async-storage/async-storage';
import { DEFAULT_BASE_URL } from '@agentvault/contract';
import type { AppSettings } from '../types';
import { updateClientConfig } from '../api/agentvault';

const SETTINGS_KEY = 'agentvault_settings';

export const DEFAULT_SETTINGS: AppSettings = {
  serverUrl: DEFAULT_BASE_URL,
  defaultProject: '',
  token: '',
};

interface SettingsContextValue {
  settings: AppSettings;
  loaded: boolean;
  saveSettings: (patch: Partial<AppSettings>) => Promise<void>;
}

const SettingsContext = createContext<SettingsContextValue | null>(null);

export function SettingsProvider({ children }: { children: React.ReactNode }) {
  const [settings, setSettings] = useState<AppSettings>(DEFAULT_APP_SETTINGS);
  const [settings, setSettings] = useState<AppSettings>(DEFAULT_SETTINGS);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let mounted = true;
    loadSettings()
      .then((next) => {
        if (!mounted) return;
    AsyncStorage.getItem(SETTINGS_KEY)
      .then((data) => {
        if (!mounted) return;
        const parsed: Partial<AppSettings> | null = data ? JSON.parse(data) : null;
        const next: AppSettings = { ...DEFAULT_SETTINGS, ...parsed };
        setSettings(next);
        updateClientConfig(next.serverUrl, next.token);
      })
      .catch(() => {
        if (mounted) {
          updateClientConfig(DEFAULT_APP_SETTINGS.serverUrl, DEFAULT_APP_SETTINGS.token);
          updateClientConfig(DEFAULT_SETTINGS.serverUrl, DEFAULT_SETTINGS.token);
        }
      })
      .finally(() => {
        if (mounted) setLoaded(true);
      });
    return () => {
      mounted = false;
    };
  }, []);

  const saveSettings = useCallback(
    async (patch: Partial<AppSettings>) => {
      const next: AppSettings = { ...settings, ...patch };
      await persistSettings(next);
      setSettings(next);
      updateClientConfig(next.serverUrl, next.token);
    },
    [settings],
      await AsyncStorage.setItem(SETTINGS_KEY, JSON.stringify(next));
      setSettings(next);
      updateClientConfig(next.serverUrl, next.token);
    },
    [settings]
  );

  return (
    <SettingsContext.Provider value={{ settings, loaded, saveSettings }}>
      {children}
    </SettingsContext.Provider>
  );
}

export function useSettings(): SettingsContextValue {
  const ctx = useContext(SettingsContext);
  if (!ctx) {
    throw new Error('useSettings must be used within a SettingsProvider');
  }
  return ctx;
}
