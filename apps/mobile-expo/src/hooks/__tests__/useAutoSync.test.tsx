import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import { AppState, type AppStateStatus } from 'react-native';
import { syncCaptures } from '../../storage/sync';
import { useConnection } from '../useConnection';
import { useAutoSync } from '../useAutoSync';

jest.mock('../../storage/sync', () => ({
  syncCaptures: jest.fn().mockResolvedValue({ added: 0, updated: 0, failed: 0, removed: 0 }),
}));

jest.mock('../useConnection', () => ({
  useConnection: jest.fn(),
}));

const activeHooks: Array<() => void> = [];

afterEach(() => {
  while (activeHooks.length) {
    const unmount = activeHooks.pop();
    unmount?.();
  }
});

function renderHook<T>(hook: () => T): {
  result: { current: T };
  rerender: () => void;
  unmount: () => void;
} {
  const result = { current: undefined as unknown as T };
  function TestComponent() {
    result.current = hook();
    return null;
  }
  let renderer: TestRenderer.ReactTestRenderer;
  act(() => {
    renderer = TestRenderer.create(<TestComponent />);
  });
  const unmount = () => act(() => renderer.unmount());
  activeHooks.push(unmount);
  return {
    result,
    rerender: () => act(() => renderer.update(<TestComponent />)),
    unmount,
  };
}

describe('useAutoSync', () => {
  let appStateHandlers: Set<(state: AppStateStatus) => void>;
  let addEventListenerSpy: jest.SpyInstance;

  const emitAppState = (state: AppStateStatus) => {
    appStateHandlers.forEach((handler) => handler(state));
  };

  beforeEach(() => {
    jest.clearAllMocks();
    appStateHandlers = new Set();
    addEventListenerSpy = jest
      .spyOn(AppState, 'addEventListener')
      .mockImplementation((event, handler: unknown) => {
        if (event === 'change') appStateHandlers.add(handler as (state: AppStateStatus) => void);
        return { remove: () => appStateHandlers.delete(handler as (state: AppStateStatus) => void) } as never;
      });
    (useConnection as jest.Mock).mockReturnValue({ status: 'online', check: jest.fn() });
  });

  afterEach(() => {
    addEventListenerSpy.mockRestore();
  });

  it('does not sync immediately on mount', async () => {
    renderHook(() => useAutoSync());
    await act(async () => {
      await Promise.resolve();
    });
    expect(syncCaptures).not.toHaveBeenCalled();
  });

  it('syncs when the app returns to the foreground', async () => {
    renderHook(() => useAutoSync());
    await act(async () => {
      await Promise.resolve();
    });

    act(() => {
      emitAppState('background');
    });
    await act(async () => {
      await Promise.resolve();
    });
    expect(syncCaptures).not.toHaveBeenCalled();

    act(() => {
      emitAppState('active');
    });
    await act(async () => {
      await Promise.resolve();
    });
    expect(syncCaptures).toHaveBeenCalledTimes(1);
    expect(syncCaptures).toHaveBeenCalledWith({ continueOnError: true });
  });

  it('syncs when the connection recovers from offline', async () => {
    (useConnection as jest.Mock).mockReturnValue({ status: 'offline', check: jest.fn() });
    const { rerender } = renderHook(() => useAutoSync());
    await act(async () => {
      await Promise.resolve();
    });
    expect(syncCaptures).not.toHaveBeenCalled();

    (useConnection as jest.Mock).mockReturnValue({ status: 'online', check: jest.fn() });
    rerender();
    await act(async () => {
      await Promise.resolve();
    });

    expect(syncCaptures).toHaveBeenCalledTimes(1);
    expect(syncCaptures).toHaveBeenCalledWith({ continueOnError: true });
  });

  it('does not sync again while a sync is already in progress', async () => {
    let resolveSync: (value: unknown) => void = () => {};
    (syncCaptures as jest.Mock).mockReturnValueOnce(
      new Promise((resolve) => {
        resolveSync = resolve;
      }),
    );

    renderHook(() => useAutoSync());
    await act(async () => {
      await Promise.resolve();
    });

    act(() => {
      emitAppState('background');
      emitAppState('active');
    });
    await act(async () => {
      await Promise.resolve();
    });

    act(() => {
      emitAppState('background');
      emitAppState('active');
    });
    await act(async () => {
      await Promise.resolve();
    });

    expect(syncCaptures).toHaveBeenCalledTimes(1);
    resolveSync({ added: 0, updated: 0, failed: 0, removed: 0 });
  });
});
