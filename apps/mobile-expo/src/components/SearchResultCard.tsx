import React from 'react';
import { View, Text, StyleSheet, TouchableOpacity } from 'react-native';
import { colors, spacing, radii, typography } from '../theme';
import type { SearchResult } from '../types';

interface SearchResultCardProps {
  result: SearchResult;
  onPress?: (r: SearchResult) => void;
}

function getPriorityColor(snippet: string): string | null {
  const lower = snippet.toLowerCase();
  if (lower.includes('p1') || lower.includes('urgent') || lower.includes('high')) {
    return colors.error;
  }
  if (lower.includes('p2') || lower.includes('medium')) {
    return colors.warning;
  }
  if (lower.includes('p3') || lower.includes('low')) {
    return colors.success;
  }
  return null;
}

export default function SearchResultCard({ result, onPress }: SearchResultCardProps) {
  const showStatus = result.type === 'task' || result.type === 'decision';
  const priorityColor = getPriorityColor(result.snippet);

  return (
    <TouchableOpacity style={styles.card} onPress={() => onPress?.(result)} activeOpacity={0.7}>
      <View style={styles.header}>
        <Text style={styles.title} numberOfLines={1}>
          {result.title}
        </Text>
        <View style={styles.badges}>
          {showStatus && (
            <View style={styles.statusBadge}>
              <Text style={styles.statusText}>{result.status}</Text>
            </View>
          )}
          <View style={styles.typeBadge}>
            <Text style={styles.typeText}>{result.type}</Text>
          </View>
          {priorityColor && <View style={[styles.priorityDot, { backgroundColor: priorityColor }]} />}
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
  badges: {
    flexDirection: 'row',
    alignItems: 'center',
    marginLeft: 10,
  },
  typeBadge: {
    backgroundColor: colors.bgTertiary,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 3,
  },
  typeText: {
    color: colors.accent,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
    textTransform: 'uppercase',
  },
  statusBadge: {
    backgroundColor: colors.bgTertiary,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 3,
    marginRight: spacing.sm,
  },
  statusText: {
    color: colors.textSecondary,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
    textTransform: 'uppercase',
  },
  priorityDot: {
    width: 7,
    height: 7,
    borderRadius: 4,
    marginLeft: spacing.sm,
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
