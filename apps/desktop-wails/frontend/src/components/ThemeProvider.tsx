import { useTheme } from '../hooks/useTheme';
import type { ReactNode } from 'react';

interface Props {
  children: ReactNode;
}

export function ThemeProvider({ children }: Props) {
  // The hook keeps the <html> class in sync with the stored/system preference.
  useTheme();
  return <>{children}</>;
}
