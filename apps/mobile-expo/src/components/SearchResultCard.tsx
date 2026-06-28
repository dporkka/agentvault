import React from 'react';
import { View, Text, StyleSheet, TouchableOpacity } from 'react-native';
import { colors, spacing, radii, typography } from '../theme';
import type { SearchResult } from '../types';

interface SearchResultCardProps {
  result: SearchResult;
  onPress?: (r: SearchResult) => void;
}

export default function SearchResultCard({ result, onPress }: SearchResultCardProps) {
  return (
    <TouchableOpacity style={styles.card} onPress={() => onPress?.(result)} activeOpacity={0.7}>
      <View style={styles.header}>
        <Text style={styles.title} numberOfLines={1}>
          {result.title}
        </Text>
        <View style={styles.typeBadge}>
          <Text style={styles.typeText}>{result.type}</Text>
        </View>
      </View>
      <Text style={styles.snippet} numberOfLines={3}>
        {result.snippet}
      </Text>
      <Text style={styles.path}>{result.path}</Text>
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.xl,
    padding: 14,
    marginBottom: 10,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: spacing.sm,
  },
  title: {
    flex: 1,
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.semibold,
  },
  typeBadge: {
    backgroundColor: colors.bgTertiary,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 3,
    marginLeft: 10,
  },
  typeText: {
    color: colors.accent,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
    textTransform: 'uppercase',
  },
  snippet: {
    color: colors.textSecondary,
    fontSize: typography.sizes.md,
    lineHeight: 18,
    marginBottom: 6,
  },
  path: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
  },
});
