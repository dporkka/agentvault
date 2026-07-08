import { describe, it, expect, vi, beforeEach } from 'vitest';
import { renderHook, waitFor, act } from '@testing-library/react';
import { useApi, useLazyApi } from '../useApi';
import { ApiError } from '@agentvault/contract';

const mockApiCall = vi.fn();

describe('useApi', () => {
  beforeEach(() => {
    mockApiCall.mockReset();
  });

  it('starts loading and returns data on success', async () => {
    mockApiCall.mockResolvedValueOnce({ id: '1' });

    const { result } = renderHook(() => useApi(mockApiCall, [], true));

    expect(result.current.loading).toBe(true);
    expect(result.current.data).toBeNull();

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.data).toEqual({ id: '1' });
    expect(result.current.error).toBeNull();
  });

  it('does not call immediately when immediate is false', async () => {
    mockApiCall.mockResolvedValueOnce({ id: '1' });

    const { result } = renderHook(() => useApi(mockApiCall, [], false));

    expect(result.current.loading).toBe(false);
    expect(mockApiCall).not.toHaveBeenCalled();

    await waitFor(() => {
      void result.current.refetch();
    });

    await waitFor(() => expect(result.current.data).toEqual({ id: '1' }));
  });

  it('surfaces an ApiError message', async () => {
    mockApiCall.mockRejectedValueOnce(new ApiError('Unauthorized', 401));

    const { result } = renderHook(() => useApi(mockApiCall, [], true));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe('Unauthorized');
  });

  it('surfaces a generic Error message', async () => {
    mockApiCall.mockRejectedValueOnce(new Error('network failure'));

    const { result } = renderHook(() => useApi(mockApiCall, [], true));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe('network failure');
  });

  it('falls back to a generic message for non-error throws', async () => {
    mockApiCall.mockRejectedValueOnce('boom');

    const { result } = renderHook(() => useApi(mockApiCall, [], true));

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe('An unexpected error occurred');
  });

  it('refetch re-executes the api call', async () => {
    mockApiCall
      .mockResolvedValueOnce({ id: '1' })
      .mockResolvedValueOnce({ id: '2' });

    const { result } = renderHook(() => useApi(mockApiCall, [], true));

    await waitFor(() => expect(result.current.data).toEqual({ id: '1' }));

    await waitFor(() => {
      void result.current.refetch();
    });

    await waitFor(() => expect(result.current.data).toEqual({ id: '2' }));
    expect(mockApiCall).toHaveBeenCalledTimes(2);
  });
});

describe('useLazyApi', () => {
  beforeEach(() => {
    mockApiCall.mockReset();
  });

  it('is idle until execute is called', () => {
    const { result } = renderHook(() => useLazyApi<{ id: string }>());

    expect(result.current.loading).toBe(false);
    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('returns data and sets state on success', async () => {
    mockApiCall.mockResolvedValueOnce({ id: '1' });

    const { result } = renderHook(() => useLazyApi<{ id: string }>());

    let returnValue: { id: string } | undefined;
    await waitFor(async () => {
      returnValue = await result.current.execute(mockApiCall);
    });

    expect(returnValue).toEqual({ id: '1' });
    expect(result.current.data).toEqual({ id: '1' });
    expect(result.current.loading).toBe(false);
  });

  it('sets error and re-throws on failure', async () => {
    mockApiCall.mockRejectedValueOnce(new Error('failed'));

    const { result } = renderHook(() => useLazyApi<{ id: string }>());

    let thrown: Error | undefined;
    await act(async () => {
      try {
        await result.current.execute(mockApiCall);
      } catch (e) {
        thrown = e as Error;
      }
    });

    expect(thrown?.message).toBe('failed');
    expect(result.current.error).toBe('failed');
    expect(result.current.loading).toBe(false);
  });

  it('resets state back to idle', async () => {
    mockApiCall.mockResolvedValueOnce({ id: '1' });

    const { result } = renderHook(() => useLazyApi<{ id: string }>());

    await waitFor(async () => {
      await result.current.execute(mockApiCall);
    });

    await waitFor(() => {
      result.current.reset();
    });

    expect(result.current.data).toBeNull();
    expect(result.current.error).toBeNull();
  });
});
