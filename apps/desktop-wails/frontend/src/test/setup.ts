import '@testing-library/jest-dom/vitest';
import { cleanup } from '@testing-library/react';
import { afterEach, vi } from 'vitest';

// jsdom does not implement scrollIntoView, but the AI panel relies on it.
Element.prototype.scrollIntoView = vi.fn();

// Clear any leftover localStorage state and DOM after each test.
afterEach(() => {
  cleanup();
  localStorage.clear();
  document.documentElement.className = '';
});
