import React, { useMemo } from 'react';
import { View, Text, FlatList, RefreshControl, Alert, StyleSheet } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import type { Capture } from '../types';
import { deleteCapture } from '../storage/localInbox';
import { syncCaptures, formatSyncResult, isSyncable } from '../storage/sync';
import { useCaptures } from '../hooks/useCaptures';
import CaptureCard from '../components/CaptureCard';
import ConnectionBadge from '../components/ConnectionBadge';
import { colors, spacing, typography } from '../theme';

interface GroupedCaptures {
  date: string;
  captures: Capture[];
}

function groupByDate(captures: Capture[]): GroupedCaptures[] {
  const map = new Map<string, Capture[]>();
  for (const c of captures) {
    const date = new Date(c.createdAt).toLocaleDateString('en-US', {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
    });
    if (!map.has(date)) map.set(date, []);
    map.get(date)!.push(c);
  }
  return Array.from(map.entries()).map(([date, captures]) => ({
    date,
    captures,
  }));
}

export default function InboxScreen() {
  const { captures, loading, refresh } = useCaptures();
  const grouped = useMemo(() => groupByDate(captures), [captures]);

  const handleSync = async () => {
    await syncCaptures({ continueOnError: true });
    await refresh();
  };

  const handleDelete = (id: string) => {
    Alert.alert('Delete Capture', 'Are you sure?', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: async () => {
          await deleteCapture(id);
          refresh();
        },
      },
    ]);
  };

  const handleSyncOne = async (cap: Capture) => {
    if (!isSyncable(cap)) return;
    const result = await syncCaptures({ captureId: cap.id, continueOnError: false });
    if (result.failed > 0) {
      Alert.alert('Error', formatSyncResult(result));
    }
    refresh();
  };

  return (
    <SafeAreaView style={styles.container} edges={['top', 'left', 'right']}>
      <View style={styles.header}>
        <View>
          <Text style={styles.title}>Inbox</Text>
          <Text style={styles.subtitle}>
            {grouped.reduce((sum, g) => sum + g.captures.length, 0)} captures
          </Text>
        </View>
        <ConnectionBadge />
      </View>

      <FlatList
        data={grouped}
        keyExtractor={(item) => item.date}
        refreshControl={
          <RefreshControl refreshing={loading} onRefresh={handleSync} tintColor={colors.accent} />
        }
        renderItem={({ item: group }) => (
          <View style={styles.group}>
            <Text style={styles.groupDate}>{group.date}</Text>
            {group.captures.map((cap) => (
              <CaptureCard
                key={cap.id}
                capture={cap}
                onDelete={handleDelete}
                onPress={() => handleSyncOne(cap)}
              />
            ))}
          </View>
        )}
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyText}>Inbox is empty</Text>
            <Text style={styles.emptySub}>Captures are saved here for offline access</Text>
          </View>
        }
        contentContainerStyle={grouped.length === 0 ? styles.emptyContainer : undefined}
      />
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.bgPrimary,
    padding: spacing.lg,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    marginBottom: spacing.md,
    marginTop: spacing.sm,
  },
  title: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xxl,
    fontWeight: typography.weights.extrabold,
  },
  subtitle: {
    color: colors.textMuted,
    fontSize: typography.sizes.md,
    marginTop: 2,
  },
  group: {
    marginBottom: spacing.sm,
  },
  groupDate: {
    color: colors.textMuted,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.semibold,
    marginBottom: spacing.sm,
    marginTop: spacing.xs,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  emptyContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  empty: {
    alignItems: 'center',
    paddingVertical: 60,
  },
  emptyText: {
    color: colors.textMuted,
    fontSize: 16,
    fontWeight: typography.weights.semibold,
  },
  emptySub: {
    color: '#4b5563',
    fontSize: typography.sizes.md,
    marginTop: 6,
    textAlign: 'center',
    paddingHorizontal: 40,
  },
});
