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
