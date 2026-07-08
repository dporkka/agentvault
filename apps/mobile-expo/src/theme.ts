// Central theme tokens for AgentVault Mobile.
// Keep this file dependency-free so it can be imported anywhere.

export const colors = {
  // Backgrounds
  bgPrimary: '#0f1117',
  bgSecondary: '#1a1d27',
  bgTertiary: '#232734',
  bgHover: '#2a2e3b',

  // Borders
  border: '#2e3344',
  borderSubtle: '#252836',

  // Text
  textPrimary: '#e4e6eb',
  textSecondary: '#9ca3af',
  textMuted: '#6b7280',

  // Text on colored/filled backgrounds
  textInverse: '#ffffff',

  // Accents
  accent: '#4f7cff',
  accentHover: '#6b93ff',
  accentMuted: 'rgba(79, 124, 255, 0.15)',

  // Status
  success: '#22c55e',
  successMuted: 'rgba(34, 197, 94, 0.2)',
  warning: '#f59e0b',
  error: '#ef4444',
  errorMuted: 'rgba(239, 68, 68, 0.13)',
  info: '#4f7cff',

  // Overlays
  overlay: 'rgba(0, 0, 0, 0.5)',
};

export const spacing = {
  xs: 4,
  sm: 8,
  md: 12,
  lg: 16,
  xl: 20,
  xxl: 24,
};

export const radii = {
  sm: 6,
  md: 8,
  lg: 10,
  xl: 12,
  xxl: 20,
};

export const typography = {
  sizes: {
    xs: 11,
    sm: 12,
    md: 13,
    base: 14,
    lg: 15,
    xl: 17,
    xxl: 22,
    xxxl: 24,
  },
  weights: {
    normal: '400',
    medium: '500',
    semibold: '600',
    bold: '700',
    extrabold: '800',
  } as const,
};

export const layout = {
  tabBarHeight: 60,
  maxSheetHeight: '60%',
} as const;

export const semanticTypeColors = {
  note: { bg: 'rgba(34, 197, 94, 0.13)', text: '#4ade80' },
  decision: { bg: 'rgba(245, 158, 11, 0.13)', text: '#fbbf24' },
  task: { bg: 'rgba(59, 130, 246, 0.13)', text: '#60a5fa' },
  meeting: { bg: 'rgba(168, 85, 247, 0.13)', text: '#c084fc' },
  source: { bg: 'rgba(244, 63, 94, 0.13)', text: '#fb7185' },
} as const;

export function getSemanticTypeColor(type?: string) {
  switch (type?.toLowerCase()) {
    case 'note':
      return semanticTypeColors.note;
    case 'decision':
      return semanticTypeColors.decision;
    case 'task':
      return semanticTypeColors.task;
    case 'meeting':
      return semanticTypeColors.meeting;
    case 'source':
      return semanticTypeColors.source;
    default:
      return { bg: colors.bgTertiary, text: colors.textSecondary };
  }
}
