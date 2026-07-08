import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { renderHook, act } from '@testing-library/react';
import { useTheme } from '../useTheme';

const STORAGE_KEY = 'agentvault-theme';

describe('useTheme', () => {
  let mediaListeners: Array<(event: MediaQueryListEvent) => void> = [];
  let prefersDark = false;

  beforeEach(() => {
    localStorage.clear();
    document.documentElement.className = '';
    mediaListeners = [];
    prefersDark = false;

    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn((query: string) => ({
        matches: query === '(prefers-color-scheme: dark)' && prefersDark,
        media: query,
        addEventListener: (_event: string, handler: (e: MediaQueryListEvent) => void) => {
          mediaListeners.push(handler);
        },
        removeEventListener: (_event: string, handler: (e: MediaQueryListEvent) => void) => {
          mediaListeners = mediaListeners.filter((l) => l !== handler);
        },
        dispatchEvent: vi.fn(),
      })),
    });
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('defaults to system theme when no stored value exists', () => {
    const { result } = renderHook(() => useTheme());
    expect(result.current.theme).toBe('system');
    expect(result.current.resolved).toBe('light');
  });

  it('reads the stored theme on mount', () => {
    localStorage.setItem(STORAGE_KEY, 'dark');
    const { result } = renderHook(() => useTheme());
    expect(result.current.theme).toBe('dark');
    expect(result.current.resolved).toBe('dark');
  });

  it('ignores invalid stored values and falls back to system', () => {
    localStorage.setItem(STORAGE_KEY, 'invalid');
    const { result } = renderHook(() => useTheme());
    expect(result.current.theme).toBe('system');
    expect(result.current.resolved).toBe('light');
  });

  it('applies the resolved theme class on mount', () => {
    localStorage.setItem(STORAGE_KEY, 'dark');
    renderHook(() => useTheme());
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('updates theme, resolved, and localStorage when setTheme is called', () => {
    const { result } = renderHook(() => useTheme());

    act(() => result.current.setTheme('dark'));

    expect(result.current.theme).toBe('dark');
    expect(result.current.resolved).toBe('dark');
    expect(localStorage.getItem(STORAGE_KEY)).toBe('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('resolves system theme to light by default', () => {
    const { result } = renderHook(() => useTheme());
    expect(result.current.resolved).toBe('light');
  });

  it('resolves system theme to dark when the OS prefers dark', () => {
    prefersDark = true;
    const { result } = renderHook(() => useTheme());
    expect(result.current.resolved).toBe('dark');
  });

  it('reacts to system theme changes while in system mode', () => {
    const { result } = renderHook(() => useTheme());
    expect(result.current.resolved).toBe('light');

    prefersDark = true;
    act(() => {
      mediaListeners.forEach((l) =>
        l({ matches: true } as MediaQueryListEvent),
      );
    });

    expect(result.current.resolved).toBe('dark');
    expect(document.documentElement.classList.contains('dark')).toBe(true);
  });

  it('does not react to system theme changes when a fixed theme is active', () => {
    const { result } = renderHook(() => useTheme());
    act(() => result.current.setTheme('light'));

    prefersDark = true;
    act(() => {
      mediaListeners.forEach((l) =>
        l({ matches: true } as MediaQueryListEvent),
      );
    });

    expect(result.current.theme).toBe('light');
    expect(result.current.resolved).toBe('light');
    expect(document.documentElement.classList.contains('dark')).toBe(false);
  });

  it('cycles through all three theme values', () => {
    const { result } = renderHook(() => useTheme());

    act(() => result.current.setTheme('dark'));
    expect(result.current.theme).toBe('dark');

    act(() => result.current.setTheme('light'));
    expect(result.current.theme).toBe('light');

    act(() => result.current.setTheme('system'));
    expect(result.current.theme).toBe('system');
  });

  it('silently ignores localStorage write errors', () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceeded');
    });

    const { result } = renderHook(() => useTheme());
    expect(() => act(() => result.current.setTheme('dark'))).not.toThrow();
    expect(result.current.theme).toBe('dark');

    setItem.mockRestore();
  });

  it('cleans up media query listeners on unmount', () => {
    const { unmount } = renderHook(() => useTheme());
    expect(mediaListeners.length).toBeGreaterThan(0);
    unmount();
    expect(mediaListeners).toHaveLength(0);
  });
});
