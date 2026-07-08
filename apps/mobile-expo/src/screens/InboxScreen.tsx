import React, { useMemo, useState } from 'react';
import { View, Text, FlatList, RefreshControl, Alert, StyleSheet, TouchableOpacity } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import type { Capture } from '../types';
import { deleteCapture } from '../storage/localInbox';
import { syncCaptures, formatSyncResult, isSyncable, canRetry } from '../storage/sync';
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

type Filter = 'all' | 'pending';

export default function InboxScreen() {
  const { captures, loading, refresh } = useCaptures();
  const [filter, setFilter] = useState<Filter>('all');

  const filteredCaptures = useMemo(() => {
    if (filter === 'pending') {
      return captures.filter((c) => !c.synced && c.syncStatus !== 'syncing');
    }
    return captures;
  }, [captures, filter]);

  const grouped = useMemo(() => groupByDate(filteredCaptures), [filteredCaptures]);

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

  const handleRetryOne = async (cap: Capture) => {
    if (!canRetry(cap)) {
      Alert.alert('Retry', 'Please wait before retrying this capture.');
      return;
    }
    const result = await syncCaptures({ captureId: cap.id, continueOnError: false, force: true });
    if (result.failed > 0) {
      Alert.alert('Retry failed', formatSyncResult(result));
    }
    refresh();
  };

  const handleRetryAll = async () => {
    const pending = captures.filter((c) => canRetry(c));
    if (pending.length === 0) {
      Alert.alert('Retry', 'No captures ready to retry right now.');
      return;
    }
    const result = await syncCaptures({ continueOnError: true, force: true });
    Alert.alert('Retry result', formatSyncResult(result));
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
        <View style={styles.headerActions}>
          <TouchableOpacity
            style={[styles.filterBtn, filter === 'pending' && styles.filterBtnActive]}
            onPress={() => setFilter((f) => (f === 'all' ? 'pending' : 'all'))}
          >
            <Text style={[styles.filterText, filter === 'pending' && styles.filterTextActive]}>
              {filter === 'pending' ? 'Pending' : 'All'}
            </Text>
          </TouchableOpacity>
          <TouchableOpacity style={styles.retryAllBtn} onPress={handleRetryAll}>
            <Text style={styles.retryAllText}>Retry all</Text>
          </TouchableOpacity>
          <ConnectionBadge />
        </View>
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
                onRetry={handleRetryOne}
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
  headerActions: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  filterBtn: {
    borderWidth: 1,
    borderColor: colors.borderSubtle,
    borderRadius: 6,
    paddingHorizontal: spacing.sm,
    paddingVertical: 4,
  },
  filterBtnActive: {
    backgroundColor: colors.accent,
    borderColor: colors.accent,
  },
  filterText: {
    color: colors.textSecondary,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
  },
  filterTextActive: {
    color: '#fff',
  },
  retryAllBtn: {
    borderWidth: 1,
    borderColor: colors.warning,
    borderRadius: 6,
    paddingHorizontal: spacing.sm,
    paddingVertical: 4,
  },
  retryAllText: {
    color: colors.warning,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
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
