import React, { useMemo, useState, useCallback, memo } from 'react';
import { View, Text, FlatList, RefreshControl, Alert, StyleSheet, TouchableOpacity } from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import Ionicons from '@expo/vector-icons/Ionicons';
import type { Capture } from '../types';
import { deleteCapture } from '../storage/localInbox';
import { syncCaptures, formatSyncResult, isSyncable, canRetry } from '../storage/sync';
import { useCaptures } from '../hooks/useCaptures';
import CaptureCard from '../components/CaptureCard';
import ConnectionBadge from '../components/ConnectionBadge';
import { colors, spacing, radii, typography } from '../theme';

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

type ListItem =
  | { type: 'header'; id: string; date: string }
  | { type: 'capture'; id: string; capture: Capture };

interface HeaderRowProps {
  date: string;
}

const HeaderRow = memo(function HeaderRow({ date }: HeaderRowProps) {
  return <Text style={styles.groupDate}>{date}</Text>;
});

interface CaptureRowProps {
  capture: Capture;
  onDelete: (id: string) => void;
  onSync: (cap: Capture) => void;
  onRetry: (cap: Capture) => void;
  selectMode?: boolean;
  selected?: boolean;
  onToggleSelect?: (id: string) => void;
}

const CaptureRow = memo(function CaptureRow({ capture, onDelete, onSync, onRetry, selectMode, selected, onToggleSelect }: CaptureRowProps) {
  return (
    <View style={selectMode && styles.selectRow}>
      {selectMode && (
        <TouchableOpacity
          style={[styles.checkbox, selected && styles.checkboxActive]}
          onPress={() => onToggleSelect?.(capture.id)}
          accessibilityRole="checkbox"
          accessibilityState={{ checked: selected }}
        >
          {selected && <Ionicons name="checkmark" size={14} color={colors.textPrimary} />}
        </TouchableOpacity>
      )}
      <View style={selectMode && styles.selectCardFlex}>
        <CaptureCard
          capture={capture}
          onDelete={onDelete}
          onPress={selectMode ? undefined : () => onSync(capture)}
          onRetry={onRetry}
        />
      </View>
    </View>
  );
});


export default function InboxScreen() {
  const { captures, loading, refresh } = useCaptures();
  const [filter, setFilter] = useState<Filter>('all');
  const [syncing, setSyncing] = useState(false);
  const [selectMode, setSelectMode] = useState(false);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  const filteredCaptures = useMemo(() => {
    if (filter === 'pending') {
      return captures.filter((c) => !c.synced && c.syncStatus !== 'syncing');
    }
    return captures;
  }, [captures, filter]);

  const grouped = useMemo(() => groupByDate(filteredCaptures), [filteredCaptures]);

  const items = useMemo<ListItem[]>(() => {
    const out: ListItem[] = [];
    for (const g of grouped) {
      out.push({ type: 'header', id: `header-${g.date}`, date: g.date });
      for (const c of g.captures) {
        out.push({ type: 'capture', id: c.id, capture: c });
      }
    }
    return out;
  }, [grouped]);

  const handleSync = useCallback(async () => {
    setSyncing(true);
    try {
      await syncCaptures({ continueOnError: true });
    } finally {
      setSyncing(false);
      await refresh();
    }
  }, [refresh]);

  const handleDelete = useCallback((id: string) => {
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
  }, [refresh]);

  const handleSyncOne = useCallback(async (cap: Capture) => {
    if (!isSyncable(cap)) return;
    const result = await syncCaptures({ captureId: cap.id, continueOnError: false });
    if (result.failed > 0) {
      Alert.alert('Error', formatSyncResult(result));
    }
    refresh();
  }, [refresh]);

  const handleRetryOne = useCallback(async (cap: Capture) => {
    if (!canRetry(cap)) {
      Alert.alert('Retry', 'Please wait before retrying this capture.');
      return;
    }
    const result = await syncCaptures({ captureId: cap.id, continueOnError: false, force: true });
    if (result.failed > 0) {
      Alert.alert('Retry failed', formatSyncResult(result));
    }
    refresh();
  }, [refresh]);

  const handleRetryAll = useCallback(async () => {
    const pending = captures.filter((c) => canRetry(c));
    if (pending.length === 0) {
      Alert.alert('Retry', 'No captures ready to retry right now.');
      return;
    }
    const result = await syncCaptures({ continueOnError: true, force: true });
    Alert.alert('Retry result', formatSyncResult(result));
    refresh();
  }, [captures, refresh]);

  const handleToggleSelect = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      next.has(id) ? next.delete(id) : next.add(id);
      return next;
    });
  }, []);

  const handleBatchDelete = useCallback(async () => {
    const ids = [...selectedIds];
    await Promise.all(ids.map((id) => deleteCapture(id)));
    setSelectedIds(new Set());
    setSelectMode(false);
    refresh();
  }, [selectedIds, refresh]);

  const handleBatchSync = useCallback(async () => {
    const ids = [...selectedIds];
    let failed = 0;
    for (const id of ids) {
      const result = await syncCaptures({ captureId: id, continueOnError: true });
      if (result.failed > 0) failed++;
    }
    if (failed > 0) Alert.alert('Sync', `${failed} of ${ids.length} failed to sync.`);
    setSelectedIds(new Set());
    setSelectMode(false);
    refresh();
  }, [selectedIds, refresh]);

  const renderItem = useCallback(({ item }: { item: ListItem }) => {
    if (item.type === 'header') {
      return <HeaderRow date={item.date} />;
    }
    return (
      <CaptureRow
        capture={item.capture}
        onDelete={handleDelete}
        onSync={handleSyncOne}
        onRetry={handleRetryOne}
        selectMode={selectMode}
        selected={selectedIds.has(item.capture.id)}
        onToggleSelect={handleToggleSelect}
      />
    );
  }, [handleDelete, handleSyncOne, handleRetryOne, selectMode, selectedIds, handleToggleSelect]);

  const keyExtractor = useCallback((item: ListItem) => item.id, []);

  const totalCaptures = useMemo(
    () => grouped.reduce((sum, g) => sum + g.captures.length, 0),
    [grouped],
  );

  return (
    <SafeAreaView style={styles.container} edges={['top', 'left', 'right']}>
      <View style={styles.header}>
        <View>
          <Text style={styles.title}>Inbox</Text>
          <Text style={styles.subtitle}>{totalCaptures} captures</Text>
        </View>
        <View style={styles.headerActions}>
          <TouchableOpacity
            style={[styles.filterBtn, filter === 'pending' && styles.filterBtnActive]}
            onPress={() => setFilter((f) => (f === 'all' ? 'pending' : 'all'))}
            accessibilityRole="button"
            accessibilityLabel={filter === 'pending' ? 'Show all captures' : 'Show pending captures'}
          >
            <Text style={[styles.filterText, filter === 'pending' && styles.filterTextActive]}>
              {filter === 'pending' ? 'Pending' : 'All'}
            </Text>
          </TouchableOpacity>
          <TouchableOpacity
            style={styles.retryAllBtn}
            onPress={handleRetryAll}
            accessibilityRole="button"
            accessibilityLabel="Retry all failed captures"
          >
            <Text style={styles.retryAllText}>Retry all</Text>
          </TouchableOpacity>
          <TouchableOpacity
            style={[styles.filterBtn, selectMode && styles.filterBtnActive]}
            onPress={() => { setSelectMode((m) => !m); setSelectedIds(new Set()); }}
            accessibilityRole="button"
            accessibilityLabel={selectMode ? 'Cancel selection' : 'Select captures'}
          >
            <Text style={[styles.filterText, selectMode && styles.filterTextActive]}>
              {selectMode ? 'Cancel' : 'Select'}
            </Text>
          </TouchableOpacity>
          <ConnectionBadge />
        </View>
      </View>

      <FlatList
        data={items}
        keyExtractor={keyExtractor}
        renderItem={renderItem}
        refreshControl={
          <RefreshControl refreshing={loading || syncing} onRefresh={handleSync} tintColor={colors.accent} />
        }
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyText}>Inbox is empty</Text>
            <Text style={styles.emptySub}>Captures are saved here for offline access</Text>
          </View>
        }
        contentContainerStyle={items.length === 0 ? styles.emptyContainer : undefined}
      />

      {selectMode && (
        <View style={styles.batchBar}>
          <TouchableOpacity
            style={styles.batchDeleteBtn}
            onPress={handleBatchDelete}
            disabled={selectedIds.size === 0}
            accessibilityRole="button"
            accessibilityLabel="Delete selected captures"
          >
            <Text style={styles.batchDeleteText}>Delete ({selectedIds.size})</Text>
          </TouchableOpacity>
          <TouchableOpacity
            style={styles.batchSyncBtn}
            onPress={handleBatchSync}
            disabled={selectedIds.size === 0}
            accessibilityRole="button"
            accessibilityLabel="Sync selected captures"
          >
            <Text style={styles.batchSyncText}>Sync ({selectedIds.size})</Text>
          </TouchableOpacity>
        </View>
      )}
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
    color: colors.textPrimary,
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
    color: colors.textSecondary,
    fontSize: typography.sizes.md,
    marginTop: 6,
    textAlign: 'center',
    paddingHorizontal: 40,
  },
  batchBar: {
    flexDirection: 'row',
    gap: 10,
    paddingTop: spacing.sm,
    borderTopWidth: 1,
    borderTopColor: colors.borderSubtle,
  },
  batchDeleteBtn: {
    flex: 1,
    backgroundColor: colors.errorMuted,
    borderRadius: radii.md,
    paddingVertical: 10,
    alignItems: 'center',
    borderWidth: 1,
    borderColor: colors.error,
  },
  batchDeleteText: {
    color: colors.error,
    fontWeight: typography.weights.semibold,
    fontSize: typography.sizes.sm,
  },
  batchSyncBtn: {
    flex: 1,
    backgroundColor: colors.successMuted,
    borderRadius: radii.md,
    paddingVertical: 10,
    alignItems: 'center',
    borderWidth: 1,
    borderColor: colors.success,
  },
  batchSyncText: {
    color: colors.success,
    fontWeight: typography.weights.semibold,
    fontSize: typography.sizes.sm,
  },
  checkbox: {
    width: 22,
    height: 22,
    borderRadius: 4,
    borderWidth: 2,
    borderColor: colors.borderSubtle,
    marginRight: 10,
    marginTop: 14,
    alignItems: 'center',
    justifyContent: 'center',
  },
  checkboxActive: {
    backgroundColor: colors.accent,
    borderColor: colors.accent,
  },
  selectRow: {
    flexDirection: 'row',
    alignItems: 'flex-start',
  },
  selectCardFlex: {
    flex: 1,
  },
});
