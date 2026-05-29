/** @type {import('tailwindcss').Config} */
export default {
  darkMode: 'class',
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'system-ui', 'sans-serif'],
        mono: ['JetBrains Mono', 'monospace'],
      },
      colors: {
        vault: {
          bg: {
            primary: '#0f1117',
            secondary: '#1a1d27',
            tertiary: '#232734',
            hover: '#2a2f3d',
          },
          accent: {
            DEFAULT: '#4f7cff',
            hover: '#3d6aef',
            muted: 'rgba(79, 124, 255, 0.15)',
          },
          text: {
            primary: '#e4e6eb',
            secondary: '#9ca3af',
            muted: '#6b7280',
          },
          border: '#2e3344',
          success: '#22c55e',
          warning: '#f59e0b',
          error: '#ef4444',
        },
      },
      typography: (theme) => ({
        DEFAULT: {
          css: {
            color: theme('colors.vault.text.primary'),
            h1: { color: theme('colors.vault.text.primary'), fontWeight: '600' },
            h2: { color: theme('colors.vault.text.primary'), fontWeight: '600' },
            h3: { color: theme('colors.vault.text.primary'), fontWeight: '600' },
            h4: { color: theme('colors.vault.text.primary'), fontWeight: '600' },
            strong: { color: theme('colors.vault.text.primary') },
            code: { color: theme('colors.vault.accent.DEFAULT'), backgroundColor: theme('colors.vault.bg.tertiary'), padding: '0.2em 0.4em', borderRadius: '0.25em', fontWeight: '500' },
            pre: { backgroundColor: theme('colors.vault.bg.tertiary'), border: `1px solid ${theme('colors.vault.border')}` },
            blockquote: { borderLeftColor: theme('colors.vault.accent.DEFAULT'), color: theme('colors.vault.text.secondary') },
            a: { color: theme('colors.vault.accent.DEFAULT'), textDecoration: 'none' },
            'a:hover': { textDecoration: 'underline' },
            ul: { color: theme('colors.vault.text.primary') },
            ol: { color: theme('colors.vault.text.primary') },
            li: { color: theme('colors.vault.text.primary') },
            hr: { borderColor: theme('colors.vault.border') },
          },
        },
      }),
    },
  },
  plugins: [require('@tailwindcss/typography')],
};
