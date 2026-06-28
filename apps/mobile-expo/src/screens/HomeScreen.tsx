import React, { useState, useCallback } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  FlatList,
  RefreshControl,
  StyleSheet,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { syncCaptures, formatSyncResult } from '../storage/sync';
import { addCapture, deleteCapture } from '../storage/localInbox';
import { useCaptures } from '../hooks/useCaptures';
import CaptureCard from '../components/CaptureCard';
import ConnectionBadge from '../components/ConnectionBadge';
import { colors, spacing, radii, typography } from '../theme';
import type { RootTabScreenProps } from '../navigation/types';

export default function HomeScreen(_props: RootTabScreenProps<'Home'>) {
  const [quickText, setQuickText] = useState('');
  const { captures, loading, refresh } = useCaptures(20);
  const [syncing, setSyncing] = useState(false);
  const [message, setMessage] = useState('');

  const handleRefresh = useCallback(async () => {
    await refresh();
  }, [refresh]);

  const showMessage = (msg: string) => {
    setMessage(msg);
    setTimeout(() => setMessage(''), 2500);
  };

  const handleQuickCapture = async () => {
    const text = quickText.trim();
    if (!text) return;
    const title = text.length > 50 ? text.slice(0, 50) + '...' : text;
    await addCapture({ type: 'text', title, text, tags: ['quick'] });
    setQuickText('');
    showMessage('Saved to inbox');
    refresh();
  };

  const handleSync = async () => {
    setSyncing(true);
    const result = await syncCaptures({ continueOnError: true });
    setSyncing(false);
    showMessage(formatSyncResult(result));
    refresh();
  };

  const handleDelete = async (id: string) => {
    await deleteCapture(id);
    refresh();
  };

  return (
    <SafeAreaView style={styles.container} edges={['top', 'left', 'right']}>
      <View style={styles.header}>
        <View>
          <Text style={styles.logo}>AgentVault</Text>
          <Text style={styles.subtitle}>Capture anywhere</Text>
        </View>
        <ConnectionBadge />
      </View>

      <View style={styles.quickCapture}>
        <TextInput
          style={styles.input}
          placeholder="Quick capture... jot an idea"
          placeholderTextColor={colors.textMuted}
          value={quickText}
          onChangeText={setQuickText}
          multiline
          maxLength={500}
        />
        <View style={styles.quickActions}>
          <Text style={styles.charCount}>{quickText.length}/500</Text>
          <TouchableOpacity
            style={[styles.captureBtn, !quickText.trim() && styles.captureBtnDisabled]}
            onPress={handleQuickCapture}
            disabled={!quickText.trim()}
          >
            <Text style={styles.captureBtnText}>Capture</Text>
          </TouchableOpacity>
        </View>
      </View>

      {message ? (
        <View style={styles.toast}>
          <Text style={styles.toastText}>{message}</Text>
        </View>
      ) : null}

      <View style={styles.listHeader}>
        <Text style={styles.listTitle}>Recent Captures</Text>
        <TouchableOpacity
          style={[styles.syncBtn, syncing && styles.syncBtnActive]}
          onPress={handleSync}
          disabled={syncing}
        >
          <Text style={styles.syncBtnText}>{syncing ? 'Syncing...' : 'Sync All'}</Text>
        </TouchableOpacity>
      </View>

      <FlatList
        data={captures}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => <CaptureCard capture={item} onDelete={handleDelete} />}
        refreshControl={
          <RefreshControl
            refreshing={loading}
            onRefresh={handleRefresh}
            tintColor={colors.accent}
          />
        }
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyText}>No captures yet</Text>
            <Text style={styles.emptySub}>Type above to capture your first idea</Text>
          </View>
        }
        contentContainerStyle={captures.length === 0 ? styles.emptyContainer : undefined}
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
    marginBottom: spacing.lg,
    marginTop: spacing.sm,
  },
  logo: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xxxl,
    fontWeight: typography.weights.extrabold,
  },
  subtitle: {
    color: colors.textMuted,
    fontSize: typography.sizes.md,
    marginTop: 2,
  },
  quickCapture: {
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.xl,
    padding: 14,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
    marginBottom: spacing.md,
  },
  input: {
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    minHeight: 60,
    textAlignVertical: 'top',
    lineHeight: 20,
  },
  quickActions: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginTop: 10,
  },
  charCount: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
  },
  captureBtn: {
    backgroundColor: colors.accent,
    borderRadius: radii.md,
    paddingHorizontal: 18,
    paddingVertical: spacing.sm,
  },
  captureBtnDisabled: {
    opacity: 0.4,
  },
  captureBtnText: {
    color: '#fff',
    fontWeight: typography.weights.semibold,
    fontSize: typography.sizes.base,
  },
  toast: {
    backgroundColor: `${colors.success}33`,
    borderRadius: radii.md,
    padding: 10,
    marginBottom: 10,
    alignItems: 'center',
  },
  toastText: {
    color: colors.success,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.medium,
  },
  listHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 10,
  },
  listTitle: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xl,
    fontWeight: typography.weights.bold,
  },
  syncBtn: {
    backgroundColor: colors.successMuted,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md,
    paddingVertical: 6,
  },
  syncBtnActive: {
    opacity: 0.5,
  },
  syncBtnText: {
    color: colors.success,
    fontSize: typography.sizes.sm,
    fontWeight: typography.weights.semibold,
  },
  emptyContainer: {
    flex: 1,
    justifyContent: 'center',
    alignItems: 'center',
  },
  empty: {
    alignItems: 'center',
    paddingVertical: 40,
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
  },
});
