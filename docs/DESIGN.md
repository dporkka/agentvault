# AgentVault Design System

This document describes the shared design language used across AgentVault clients (browser extension, web app, desktop, and mobile). The goal is to keep the UI consistent, maintainable, and accessible without relying on ad-hoc styling.

## Principles

1. **Tokens first.** Colors, spacing, radii, and typography are defined as reusable tokens. Avoid hard-coding values.
2. **Classes over inline styles.** UI components use semantic CSS classes. Inline styles are reserved for truly dynamic values (e.g., progress bars, chart data).
3. **BEM-like naming.** Component styles use block-element-modifier names: `.block__element--modifier`.
4. **One source of truth per platform.** Each client has a single token file; cross-client colors must stay in sync.

## Tokens

### Browser extension

Tokens live in `apps/browser-extension/src/popup/popup.css` as CSS custom properties:

```css
--av-bg-primary: #0f1117;
--av-bg-secondary: #14161d;
--av-bg-tertiary: #1a1d27;
--av-bg-hover: #2a2d3a;
--av-border: #2a2d3a;
--av-border-subtle: #1f222b;
--av-text-primary: #e4e6eb;
--av-text-secondary: #9ca3af;
--av-text-muted: #6b7280;
--av-accent: #4f7cff;
--av-accent-hover: #6b93ff;
--av-accent-muted: rgba(79, 124, 255, 0.12);
--av-success: #22c55e;
--av-success-muted: rgba(34, 197, 94, 0.12);
--av-warning: #f59e0b;
--av-warning-muted: rgba(245, 158, 11, 0.12);
--av-error: #ef4444;
--av-error-muted: rgba(239, 68, 68, 0.12);
```

### Web / Desktop

The web app uses Tailwind with CSS variables declared in `apps/web-local/src/styles/index.css`:

```css
--bg-primary: #0f1117;
--bg-secondary: #1a1d27;
--bg-tertiary: #232734;
--bg-hover: #2a2f3d;
--accent: #4f7cff;
--accent-hover: #3d6aef;
--accent-muted: rgba(79, 124, 255, 0.15);
--text-primary: #e4e6eb;
--text-secondary: #9ca3af;
--text-muted: #6b7280;
--border-color: #2e3344;
--success: #22c55e;
--warning: #f59e0b;
--error: #ef4444;
```

Tailwind classes map to these variables (e.g. `bg-vault-bg-primary`, `text-vault-accent`).

### Mobile

Tokens live in `apps/mobile-expo/src/theme.ts`:

```ts
export const colors = {
  bgPrimary: '#0f1117',
  bgSecondary: '#1a1d27',
  bgTertiary: '#232734',
  textPrimary: '#e4e6eb',
  textSecondary: '#9ca3af',
  textMuted: '#6b7280',
  accent: '#4f7cff',
  success: '#22c55e',
  warning: '#f59e0b',
  error: '#ef4444',
  // ...
};
```

## Naming Conventions

- `.popup-header`, `.popup-header__brand`, `.popup-header__logo`
- `.popup-tabs__btn`, `.popup-tabs__btn--active`
- `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost`, `.btn-sm`, `.btn-danger`
- `.input`, `.select`, `.readonly-field`, `.readonly-field--multiline`
- `.banner`, `.banner-success`, `.banner-error`, `.banner-warning`
- `.empty-state`, `.spinner`, `.spinner--inline`
- `.result-card`, `.result-card__header`, `.result-card__title`, `.result-card__snippet`
- `.recent-card`, `.recent-card__footer`
- `.note-viewer`, `.note-viewer__title`, `.note-viewer__pre`

Modifiers describe state or appearance; elements describe children.

## Semantic Type Badges

Note types share the same color meaning everywhere:

| Type       | Background         | Text        |
|------------|-------------------|-------------|
| note       | `#22c55e22`       | `#4ade80`   |
| decision   | `#f59e0b22`       | `#fbbf24`   |
| task       | `#3b82f622`       | `#60a5fa`   |
| meeting    | `#a855f722`       | `#c084fc`   |
| source     | `#f43f5e22`       | `#fb7185`   |
| default    | muted background  | muted text  |

Web/desktop implements these as Tailwind components in `apps/web-local/src/styles/index.css` (`.type-badge-note`, `.type-badge-decision`, etc.).
Mobile uses `getSemanticTypeColor()` from `apps/mobile-expo/src/theme.ts`.
The browser extension uses `.badge` and `.tag` accent styles because its popup does not surface note-type badges today.

## Shared Components

- `EmptyState` (`apps/web-local/src/components/EmptyState.tsx`) — centered placeholder with optional icon, title, and subtitle. Used by `SearchView` and `ProjectDashboard`.

## Rules

- Do not add `style={{ ... }}` for layout, colors, spacing, or typography. Use existing classes or add new ones to the client token/stylesheet.
- Keep the extension popup stylesheet in `popup.css` only. Components do not import their own CSS files because `Popup.tsx` imports `popup.css` for the whole popup.
- When adding a new class, follow the existing naming and place it near related rules.
- Prefer existing tokens. If a design needs a new color, add it to all three platforms or document why it is client-specific.
