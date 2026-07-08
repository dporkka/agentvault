import { useState, useEffect, useCallback } from 'react';

export type Theme = 'light' | 'dark' | 'system';
export type ResolvedTheme = 'light' | 'dark';

const STORAGE_KEY = 'agentvault-theme';

function getSystemTheme(): ResolvedTheme {
  if (typeof window === 'undefined') return 'light';
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function getStoredTheme(): Theme | null {
  if (typeof window === 'undefined') return null;
  try {
    const value = localStorage.getItem(STORAGE_KEY);
    if (value === 'light' || value === 'dark' || value === 'system') {
      return value;
    }
  } catch {
    // Ignore storage errors (e.g. private browsing).
  }
  return null;
}

function resolveTheme(theme: Theme): ResolvedTheme {
  return theme === 'system' ? getSystemTheme() : theme;
}

function applyClass(resolved: ResolvedTheme): void {
  const html = document.documentElement;
  if (resolved === 'dark') {
    html.classList.add('dark');
  } else {
    html.classList.remove('dark');
  }
}

export interface UseThemeResult {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  resolved: ResolvedTheme;
}

export function useTheme(): UseThemeResult {
  const [theme, setThemeState] = useState<Theme>(() => getStoredTheme() || 'system');
  const [resolved, setResolved] = useState<ResolvedTheme>(() => resolveTheme(getStoredTheme() || 'system'));

  const setTheme = useCallback((next: Theme) => {
    const resolved = resolveTheme(next);
    applyClass(resolved);
    setThemeState(next);
    setResolved(resolved);
    try {
      localStorage.setItem(STORAGE_KEY, next);
    } catch {
      // Ignore storage errors.
    }
  }, []);

  // Keep the class in sync on mount (especially after SSR/hydration).
  useEffect(() => {
    applyClass(resolved);
  }, [resolved]);

  // React to system preference changes while in "system" mode.
  useEffect(() => {
    if (theme !== 'system') return;
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const handler = (event: MediaQueryListEvent) => {
      const resolved = event.matches ? 'dark' : 'light';
      applyClass(resolved);
      setResolved(resolved);
    };
    media.addEventListener('change', handler);
    return () => media.removeEventListener('change', handler);
  }, [theme]);

  return { theme, setTheme, resolved };
}
