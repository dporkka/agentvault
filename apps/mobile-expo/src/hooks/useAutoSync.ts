import { useEffect, useRef } from 'react';
import { AppState, type AppStateStatus } from 'react-native';
import { useConnection } from './useConnection';
import { useSettings } from '../context/SettingsContext';
import { syncCaptures } from '../storage/sync';
import type { AutoSyncInterval } from '../types';

const INTERVAL_MS: Record<AutoSyncInterval, number | null> = {
  off: null,
  '5min': 5 * 60 * 1000,
  '15min': 15 * 60 * 1000,
  '30min': 30 * 60 * 1000,
  '1hour': 60 * 60 * 1000,
};

/**
 * Automatically sync unsynced captures when the app comes to the foreground,
 * when the network connection recovers, and on a configurable interval.
 */
export function useAutoSync() {
  const { status } = useConnection();
  const { settings, loaded } = useSettings();
  const wasOfflineRef = useRef(status === 'offline');
  const isSyncingRef = useRef(false);

  const doSync = () => {
    if (isSyncingRef.current) return;
    isSyncingRef.current = true;
    syncCaptures({ continueOnError: true })
      .catch(() => {
        // Errors are recorded per-capture; suppress here.
      })
      .finally(() => {
        isSyncingRef.current = false;
      });
  };

  // Sync when returning from background.
  useEffect(() => {
    const handleChange = (next: AppStateStatus) => {
      if (next !== 'active' || isSyncingRef.current) return;
      doSync();
    };
    const sub = AppState.addEventListener('change', handleChange);
    return () => sub.remove();
  }, []);

  // Sync when connection recovers.
  useEffect(() => {
    if (status !== 'online') {
      wasOfflineRef.current = status === 'offline';
      return;
    }
    if (!wasOfflineRef.current || isSyncingRef.current) return;
    doSync();
    wasOfflineRef.current = false;
  }, [status]);

  // Configurable interval sync.
  useEffect(() => {
    if (!loaded) return;
    const ms = INTERVAL_MS[settings.autoSyncInterval];
    if (ms === null) return;

    const id = setInterval(doSync, ms);
    return () => clearInterval(id);
  }, [loaded, settings.autoSyncInterval]);
}
