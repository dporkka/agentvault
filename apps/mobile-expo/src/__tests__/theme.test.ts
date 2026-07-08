import { getSemanticTypeColor, colors } from '../theme';

describe('getSemanticTypeColor', () => {
  it('returns colors for known semantic types', () => {
    expect(getSemanticTypeColor('note')).toEqual({
      bg: 'rgba(34, 197, 94, 0.13)',
      text: '#4ade80',
    });
    expect(getSemanticTypeColor('decision')).toEqual({
      bg: 'rgba(245, 158, 11, 0.13)',
      text: '#fbbf24',
    });
    expect(getSemanticTypeColor('task')).toEqual({
      bg: 'rgba(59, 130, 246, 0.13)',
      text: '#60a5fa',
    });
    expect(getSemanticTypeColor('meeting')).toEqual({
      bg: 'rgba(168, 85, 247, 0.13)',
      text: '#c084fc',
    });
    expect(getSemanticTypeColor('source')).toEqual({
      bg: 'rgba(244, 63, 94, 0.13)',
      text: '#fb7185',
    });
  });

  it('is case-insensitive', () => {
    expect(getSemanticTypeColor('NOTE')).toEqual(getSemanticTypeColor('note'));
    expect(getSemanticTypeColor('Decision')).toEqual(getSemanticTypeColor('decision'));
  });

  it('returns a fallback for unknown or missing types', () => {
    const fallback = { bg: colors.bgTertiary, text: colors.textSecondary };
    expect(getSemanticTypeColor('unknown')).toEqual(fallback);
    expect(getSemanticTypeColor(undefined)).toEqual(fallback);
  });
});
