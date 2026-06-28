import { useState, useCallback } from 'react';
import { useFocusEffect } from '@react-navigation/native';
import type { Capture } from '../types';
import { getCaptures } from '../storage/localInbox';

interface UseCapturesResult {
  captures: Capture[];
  loading: boolean;
  refresh: () => Promise<void>;
}

export function useCaptures(limit?: number): UseCapturesResult {
  const [captures, setCaptures] = useState<Capture[]>([]);
  const [loading, setLoading] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const list = await getCaptures();
      setCaptures(limit ? list.slice(0, limit) : list);
    } finally {
      setLoading(false);
    }
  }, [limit]);

  useFocusEffect(
    useCallback(() => {
      refresh();
    }, [refresh]),
  );

  return { captures, loading, refresh };
}
