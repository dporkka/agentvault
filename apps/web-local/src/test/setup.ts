import '@testing-library/jest-dom/vitest';
import { vi } from 'vitest';

// jsdom does not implement scrollIntoView; silence any component that calls it.
Element.prototype.scrollIntoView = vi.fn();
