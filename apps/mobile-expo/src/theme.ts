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

  // Accents
  accent: '#4f7cff',
  accentHover: '#6b93ff',
  accentMuted: '#4f7cff33',

  // Status
  success: '#22c55e',
  successMuted: '#22c55e22',
  warning: '#f59e0b',
  error: '#ef4444',
  errorMuted: '#ef444422',
  info: '#4f7cff',
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
  note: { bg: '#22c55e22', text: '#4ade80' },
  decision: { bg: '#f59e0b22', text: '#fbbf24' },
  task: { bg: '#3b82f622', text: '#60a5fa' },
  meeting: { bg: '#a855f722', text: '#c084fc' },
  source: { bg: '#f43f5e22', text: '#fb7185' },
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
