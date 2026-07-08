import React from 'react';
import { View, Text, StyleSheet, TouchableOpacity } from 'react-native';
import Ionicons from '@expo/vector-icons/Ionicons';
import { colors, spacing, radii, typography, getSemanticTypeColor } from '../theme';
import type { Capture } from '../types';

interface CaptureCardProps {
  capture: Capture;
  onPress?: (c: Capture) => void;
  onDelete?: (id: string) => void;
  onRetry?: (c: Capture) => void;
}

const TYPE_LABELS: Record<string, string> = {
  text: 'Aa',
  voice: 'Mic',
  photo: 'Cam',
};

const STATUS_COLORS: Record<string, string> = {
  synced: colors.success,
  syncing: colors.accent,
  failed: colors.error,
  unsynced: colors.warning,
};

const STATUS_LABELS: Record<string, string> = {
  synced: 'Synced',
  syncing: 'Syncing…',
  failed: 'Failed',
  unsynced: 'Offline',
};

export default function CaptureCard({ capture, onPress, onDelete, onRetry }: CaptureCardProps) {
  const date = new Date(capture.createdAt).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

  const status = capture.synced ? 'synced' : (capture.syncStatus ?? 'unsynced');
  const statusColor = STATUS_COLORS[status] ?? STATUS_COLORS.unsynced;
  const typeColors = getSemanticTypeColor(capture.type);

  return (
    <TouchableOpacity
      style={styles.card}
      onPress={() => onPress?.(capture)}
      onLongPress={() => onDelete?.(capture.id)}
      activeOpacity={0.7}
    >
      <View style={styles.header}>
        <View style={[styles.typeBadge, { backgroundColor: typeColors.bg }]}>
          <Text style={[styles.typeText, { color: typeColors.text }]}>{TYPE_LABELS[capture.type] || '?'}</Text>
        </View>
        <Text style={styles.title} numberOfLines={1}>
          {capture.title}
        </Text>
        <View style={[styles.syncIndicator, { backgroundColor: statusColor }]} />
        {onDelete && (
          <TouchableOpacity
            style={styles.deleteBtn}
            onPress={() => onDelete(capture.id)}
            accessibilityLabel="Delete capture"
            accessibilityRole="button"
          >
            <Ionicons name="trash-outline" size={16} color={colors.error} />
          </TouchableOpacity>
        )}
      </View>

      {capture.text ? (
        <Text style={styles.preview} numberOfLines={2}>
          {capture.text}
        </Text>
      ) : null}

      {(capture.syncError || (capture.retryCount && capture.retryCount > 0) || status === 'failed' || status === 'unsynced') && (
        <View style={styles.statusRow}>
          <Text style={[styles.statusLabel, { color: statusColor }]}>
            {STATUS_LABELS[status] ?? status}
          </Text>
          {capture.syncError ? <Text style={styles.errorText}>{capture.syncError}</Text> : null}
          {capture.retryCount && capture.retryCount > 0 && (
            <Text style={styles.attemptsText}>Attempt {capture.retryCount}</Text>
          )}
          {(status === 'failed' || status === 'unsynced') && onRetry && (
            <TouchableOpacity
              style={[styles.retryBtn, { borderColor: statusColor }]}
              onPress={() => onRetry(capture)}
              accessibilityLabel="Retry sync"
              accessibilityRole="button"
            >
              <Text style={[styles.retryText, { color: statusColor }]}>Retry</Text>
            </TouchableOpacity>
          )}
        </View>
      )}

      <View style={styles.footer}>
        {capture.project ? (
          <View style={styles.projectBadge}>
            <Text style={styles.projectText}>{capture.project}</Text>
          </View>
        ) : null}
        <View style={styles.tagsRow}>
          {capture.tags.map((tag) => (
            <View key={tag} style={styles.tag}>
              <Text style={styles.tagText}>{tag}</Text>
            </View>
          ))}
        </View>
        <Text style={styles.date}>{date}</Text>
      </View>
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
  typeBadge: {
    backgroundColor: colors.bgTertiary,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 3,
    marginRight: 10,
  },
  typeText: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.bold,
  },
  title: {
    flex: 1,
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.semibold,
  },
  syncIndicator: {
    width: spacing.sm,
    height: spacing.sm,
    borderRadius: 4,
    marginLeft: spacing.sm,
  },
  deleteBtn: {
    padding: spacing.xs,
    marginLeft: spacing.xs,
  },
  preview: {
    color: colors.textMuted,
    fontSize: typography.sizes.md,
    lineHeight: 18,
    marginBottom: spacing.sm,
  },
  footer: {
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: 6,
  },
  projectBadge: {
    backgroundColor: colors.bgTertiary,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 3,
    borderWidth: 1,
    borderColor: colors.accentMuted,
  },
  projectText: {
    color: colors.accent,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
  },
  tagsRow: {
    flexDirection: 'row',
    gap: 6,
  },
  tag: {
    backgroundColor: colors.borderSubtle,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 3,
  },
  tagText: {
    color: colors.textSecondary,
    fontSize: typography.sizes.xs,
  },
  date: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
    marginLeft: 'auto',
  },
  statusRow: {
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: 6,
    marginBottom: spacing.sm,
  },
  statusLabel: {
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.bold,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  errorText: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
    flexShrink: 1,
  },
  attemptsText: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
  },
  retryBtn: {
    borderWidth: 1,
    borderRadius: radii.sm,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    marginLeft: 'auto',
  },
  retryText: {
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.bold,
  },
});
