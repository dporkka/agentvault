import React from 'react';
import { View, Text, StyleSheet, TouchableOpacity } from 'react-native';
import type { Capture } from '../types';

interface CaptureCardProps {
  capture: Capture;
  onPress?: (c: Capture) => void;
  onDelete?: (id: string) => void;
}

const TYPE_LABELS: Record<string, string> = {
  text: 'Aa',
  voice: 'Mic',
  photo: 'Cam',
};

const STATUS_COLORS: Record<string, string> = {
  synced: '#22c55e',
  syncing: '#4f7cff',
  failed: '#ef4444',
  unsynced: '#f59e0b',
};

export default function CaptureCard({ capture, onPress, onDelete }: CaptureCardProps) {
  const date = new Date(capture.createdAt).toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });

  const status = capture.synced ? 'synced' : (capture.syncStatus ?? 'unsynced');
  const statusColor = STATUS_COLORS[status] ?? STATUS_COLORS.unsynced;

  return (
    <TouchableOpacity
      style={styles.card}
      onPress={() => onPress?.(capture)}
      onLongPress={() => onDelete?.(capture.id)}
      activeOpacity={0.7}
    >
      <View style={styles.header}>
        <View style={styles.typeBadge}>
          <Text style={styles.typeText}>{TYPE_LABELS[capture.type] || '?'}</Text>
        </View>
        <Text style={styles.title} numberOfLines={1}>
          {capture.title}
        </Text>
        <View
          style={[
            styles.syncIndicator,
            { backgroundColor: statusColor },
          ]}
        />
      </View>

      {capture.text ? (
        <Text style={styles.preview} numberOfLines={2}>
          {capture.text}
        </Text>
      ) : null}

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
    backgroundColor: '#1a1d27',
    borderRadius: 12,
    padding: 14,
    marginBottom: 10,
    borderWidth: 1,
    borderColor: '#252836',
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 8,
  },
  typeBadge: {
    backgroundColor: '#4f7cff',
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 3,
    marginRight: 10,
  },
  typeText: {
    color: '#fff',
    fontSize: 11,
    fontWeight: '700',
  },
  title: {
    flex: 1,
    color: '#e4e6eb',
    fontSize: 15,
    fontWeight: '600',
  },
  syncIndicator: {
    width: 8,
    height: 8,
    borderRadius: 4,
    marginLeft: 8,
  },
  preview: {
    color: '#6b7280',
    fontSize: 13,
    lineHeight: 18,
    marginBottom: 8,
  },
  footer: {
    flexDirection: 'row',
    alignItems: 'center',
    flexWrap: 'wrap',
    gap: 6,
  },
  projectBadge: {
    backgroundColor: '#2a2f3f',
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 3,
    borderWidth: 1,
    borderColor: '#4f7cff33',
  },
  projectText: {
    color: '#4f7cff',
    fontSize: 11,
    fontWeight: '600',
  },
  tagsRow: {
    flexDirection: 'row',
    gap: 6,
  },
  tag: {
    backgroundColor: '#252836',
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 3,
  },
  tagText: {
    color: '#9ca3af',
    fontSize: 11,
  },
  date: {
    color: '#6b7280',
    fontSize: 11,
    marginLeft: 'auto',
  },
});
