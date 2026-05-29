import { useState, useEffect, useCallback, useRef } from 'react';
import { ApiError } from '@/api/client';

interface UseApiResult<T> {
  data: T | null;
  loading: boolean;
  error: string | null;
  refetch: () => void;
}

export function useApi<T>(
  apiCall: () => Promise<T>,
  deps: unknown[] = [],
  immediate = true
): UseApiResult<T> {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(immediate);
  const [error, setError] = useState<string | null>(null);
  const apiCallRef = useRef(apiCall);
  apiCallRef.current = apiCall;

  const execute = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const result = await apiCallRef.current();
      setData(result);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('An unexpected error occurred');
      }
    } finally {
      setLoading(false);
    }
  }, []);

  const refetch = useCallback(() => {
    execute();
  }, [execute]);

  useEffect(() => {
    if (immediate) {
      execute();
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps);

  return { data, loading, error, refetch };
}

export function useLazyApi<T>() {
  const [data, setData] = useState<T | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const execute = useCallback(async (apiCall: () => Promise<T>) => {
    setLoading(true);
    setError(null);
    try {
      const result = await apiCall();
      setData(result);
      return result;
    } catch (err) {
      const message = err instanceof ApiError
        ? err.message
        : err instanceof Error
          ? err.message
          : 'An unexpected error occurred';
      setError(message);
      throw err;
    } finally {
      setLoading(false);
    }
  }, []);

  const reset = useCallback(() => {
    setData(null);
    setError(null);
  }, []);

  return { data, loading, error, execute, reset };
}
