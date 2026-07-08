import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import * as NetInfo from '@react-native-community/netinfo';
import { DEFAULT_BASE_URL } from '@agentvault/contract';
import { checkHealth } from '../../api/agentvault';
import { useConnection } from '../useConnection';

jest.mock('@react-native-community/netinfo', () => ({
  fetch: jest.fn(),
  addEventListener: jest.fn(() => jest.fn()),
}));

jest.mock('../../api/agentvault', () => ({
  checkHealth: jest.fn(),
}));

jest.mock('../../context/SettingsContext', () => {
  const { DEFAULT_BASE_URL: baseUrl } = require('@agentvault/contract');
  const settings = { serverUrl: baseUrl, token: '', defaultProject: '' };
  return {
    useSettings: jest.fn(() => ({ settings, loaded: true, saveSettings: jest.fn() })),
    SettingsProvider: ({ children }: { children: React.ReactNode }) => children,
  };
});

jest.useFakeTimers();

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

describe('useConnection', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    (NetInfo.fetch as jest.Mock).mockResolvedValue({ isConnected: true });
    (checkHealth as jest.Mock).mockResolvedValue(true);
  });

  it('starts in the checking state', () => {
    const { result } = renderHook(() => useConnection(5000));
    expect(result.current.status).toBe('checking');
  });

  it('marks online when the network and server health check succeed', async () => {
    const { result } = renderHook(() => useConnection(5000));
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.status).toBe('online');
  });

  it('marks offline when the network is down', async () => {
    (NetInfo.fetch as jest.Mock).mockResolvedValue({ isConnected: false });
    const { result } = renderHook(() => useConnection(5000));
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.status).toBe('offline');
  });

  it('marks offline when the server health check fails', async () => {
    (checkHealth as jest.Mock).mockResolvedValue(false);
    const { result } = renderHook(() => useConnection(5000));
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.status).toBe('offline');
  });

  it('polls health on the configured interval', async () => {
    const { result } = renderHook(() => useConnection(5000));
    await act(async () => {
      await Promise.resolve();
    });
    expect(checkHealth).toHaveBeenCalledTimes(1);
    expect(result.current.status).toBe('online');

    (checkHealth as jest.Mock).mockClear();
    await act(async () => {
      jest.advanceTimersByTime(5000);
      jest.runAllTicks();
    });

    expect(checkHealth).toHaveBeenCalledTimes(1);
  });

  it('exposes a manual check that returns the health result', async () => {
    (checkHealth as jest.Mock).mockResolvedValueOnce(false).mockResolvedValueOnce(true);
    const { result } = renderHook(() => useConnection(5000));
    await act(async () => {
      await Promise.resolve();
    });

    let healthy: boolean | undefined;
    await act(async () => {
      healthy = await result.current.check();
    });

    expect(healthy).toBe(true);
  });
});
