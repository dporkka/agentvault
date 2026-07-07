import React, { createContext, useContext, useEffect, useState, useCallback } from 'react';
import type { AppSettings } from '../types';
import { updateClientConfig } from '../api/agentvault';
import { DEFAULT_APP_SETTINGS, loadSettings, persistSettings } from '../storage/settingsStore';

export { DEFAULT_APP_SETTINGS as DEFAULT_SETTINGS };

interface SettingsContextValue {
  settings: AppSettings;
  loaded: boolean;
  saveSettings: (patch: Partial<AppSettings>) => Promise<void>;
}

const SettingsContext = createContext<SettingsContextValue | null>(null);

export function SettingsProvider({ children }: { children: React.ReactNode }) {
  const [settings, setSettings] = useState<AppSettings>(DEFAULT_APP_SETTINGS);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let mounted = true;
    loadSettings()
      .then((next) => {
        if (!mounted) return;
        setSettings(next);
        updateClientConfig(next.serverUrl, next.token);
      })
      .catch(() => {
        if (mounted) {
          updateClientConfig(DEFAULT_APP_SETTINGS.serverUrl, DEFAULT_APP_SETTINGS.token);
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
