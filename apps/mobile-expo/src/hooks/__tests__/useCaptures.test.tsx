import React from 'react';
import TestRenderer, { act } from 'react-test-renderer';
import { useCaptures } from '../useCaptures';
import { getCaptures } from '../../storage/localInbox';
import type { Capture } from '../../types';

const focusEffectCallbacks: (() => void)[] = [];

jest.mock('@react-navigation/native', () => ({
  useFocusEffect: jest.fn((cb: () => void) => {
    focusEffectCallbacks.push(cb);
  }),
}));

jest.mock('../../storage/localInbox', () => ({
  getCaptures: jest.fn(),
}));

const activeHooks: (() => void)[] = [];

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
  act(() => {
    focusEffectCallbacks.forEach((cb) => cb());
  });
  const unmount = () => act(() => renderer.unmount());
  activeHooks.push(unmount);
  return {
    result,
    rerender: () => act(() => renderer.update(<TestComponent />)),
    unmount,
  };
}

describe('useCaptures', () => {
  beforeEach(() => {
    focusEffectCallbacks.length = 0;
  });

  const captures: Capture[] = [
    { id: '1', type: 'text', title: 'First', tags: [], createdAt: '', synced: false },
    { id: '2', type: 'text', title: 'Second', tags: [], createdAt: '', synced: false },
    { id: '3', type: 'text', title: 'Third', tags: [], createdAt: '', synced: false },
  ];

  beforeEach(() => {
    jest.clearAllMocks();
    (getCaptures as jest.Mock).mockResolvedValue(captures);
  });

  it('loads captures on mount', async () => {
    const { result } = renderHook(() => useCaptures());
    expect(result.current.loading).toBe(true);

    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.loading).toBe(false);
    expect(result.current.captures).toEqual(captures);
  });

  it('limits the number of captures when a limit is provided', async () => {
    const { result } = renderHook(() => useCaptures(2));

    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.captures).toHaveLength(2);
    expect(result.current.captures.map((c) => c.title)).toEqual(['First', 'Second']);
  });

  it('exposes a refresh function', async () => {
    const { result } = renderHook(() => useCaptures());

    await act(async () => {
      await Promise.resolve();
    });

    const updated: Capture[] = [
      { id: '4', type: 'text', title: 'New', tags: [], createdAt: '', synced: false },
    ];
    (getCaptures as jest.Mock).mockResolvedValue(updated);

    await act(async () => {
      await result.current.refresh();
    });

    expect(result.current.captures).toEqual(updated);
  });

  it('resets captures and loading when getCaptures fails', async () => {
    (getCaptures as jest.Mock).mockRejectedValue(new Error('boom'));

    const { result } = renderHook(() => useCaptures());
    expect(result.current.loading).toBe(true);

    await act(async () => {
      await Promise.resolve();
    });

    expect(result.current.loading).toBe(false);
    expect(result.current.captures).toEqual([]);
  });
});
