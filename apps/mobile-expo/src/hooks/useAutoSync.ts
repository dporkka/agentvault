import { useEffect, useRef } from 'react';
import { AppState, type AppStateStatus } from 'react-native';
import { useConnection } from './useConnection';
import { syncCaptures } from '../storage/sync';

/**
 * Automatically sync unsynced captures when the app comes to the foreground
 * or when the network connection recovers.
 */
export function useAutoSync() {
  const { status } = useConnection();
  const wasOfflineRef = useRef(status === 'offline');
  const isSyncingRef = useRef(false);

  // Sync when returning from background.
  useEffect(() => {
    const handleChange = (next: AppStateStatus) => {
      if (next !== 'active' || isSyncingRef.current) return;
      isSyncingRef.current = true;
      syncCaptures({ continueOnError: true })
        .catch(() => {
          // Errors are recorded per-capture; suppress here.
        })
        .finally(() => {
          isSyncingRef.current = false;
        });
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
    isSyncingRef.current = true;
    syncCaptures({ continueOnError: true })
      .catch(() => {
        // Errors are recorded per-capture; suppress here.
      })
      .finally(() => {
        isSyncingRef.current = false;
        wasOfflineRef.current = false;
      });
  }, [status]);
}
