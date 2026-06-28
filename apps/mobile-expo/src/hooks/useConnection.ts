import { useState, useEffect, useCallback } from 'react';
import NetInfo from '@react-native-community/netinfo';
import { useSettings } from '../context/SettingsContext';
import { checkHealth } from '../api/agentvault';

type ConnectionStatus = 'online' | 'offline' | 'checking';

interface UseConnectionResult {
  status: ConnectionStatus;
  check: () => Promise<boolean>;
}

export function useConnection(pollIntervalMs = 15000): UseConnectionResult {
  const { settings, loaded } = useSettings();
  const [status, setStatus] = useState<ConnectionStatus>('checking');

  const check = useCallback(async () => {
    if (!loaded) {
      setStatus('checking');
      return false;
    }
    const netInfo = await NetInfo.fetch();
    if (!netInfo.isConnected) {
      setStatus('offline');
      return false;
    }
    const healthy = await checkHealth(settings.serverUrl);
    setStatus(healthy ? 'online' : 'offline');
    return healthy;
  }, [loaded, settings.serverUrl]);

  useEffect(() => {
    let mounted = true;
    const run = async () => {
      if (!mounted) return;
      await check();
    };
    run();
    const interval = setInterval(run, pollIntervalMs);
    const unsub = NetInfo.addEventListener((state) => {
      if (!state.isConnected) setStatus('offline');
    });
    return () => {
      mounted = false;
      clearInterval(interval);
      unsub();
    };
  }, [check, pollIntervalMs]);

  return { status, check };
}
