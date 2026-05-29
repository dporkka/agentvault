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
import { useFocusEffect } from '@react-navigation/native';
import type { Capture } from '../types';
import { addCapture, getCaptures, getUnsyncedCaptures, markAsSynced, deleteCapture } from '../storage/localInbox';
import { sendCapture } from '../api/agentvault';
import CaptureCard from '../components/CaptureCard';
import ConnectionBadge from '../components/ConnectionBadge';

export default function HomeScreen({ navigation }: { navigation: any }) {
  const [quickText, setQuickText] = useState('');
  const [captures, setCaptures] = useState<Capture[]>([]);
  const [refreshing, setRefreshing] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const [message, setMessage] = useState('');

  const load = useCallback(async () => {
    const list = await getCaptures();
    setCaptures(list.slice(0, 20));
  }, []);

  const handleRefresh = useCallback(async () => {
    setRefreshing(true);
    try {
      await load();
    } finally {
      setRefreshing(false);
    }
  }, [load]);

  useFocusEffect(
    useCallback(() => {
      load();
    }, [load])
  );

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
    load();
  };

  const handleSync = async () => {
    setSyncing(true);
    const unsynced = await getUnsyncedCaptures();
    let sent = 0;
    for (const cap of unsynced) {
      try {
        await sendCapture({
          type: cap.type,
          title: cap.title,
          text: cap.text,
          project: cap.project,
          tags: cap.tags,
        });
        await markAsSynced(cap.id);
        sent++;
      } catch {
        // stop on first error, will retry later
        break;
      }
    }
    setSyncing(false);
    showMessage(sent > 0 ? `Synced ${sent} captures` : 'Nothing to sync');
    load();
  };

  const handleDelete = async (id: string) => {
    await deleteCapture(id);
    load();
  };

  return (
    <View style={styles.container}>
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
          placeholderTextColor="#6b7280"
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
          <Text style={styles.syncBtnText}>
            {syncing ? 'Syncing...' : 'Sync All'}
          </Text>
        </TouchableOpacity>
      </View>

      <FlatList
        data={captures}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => (
          <CaptureCard capture={item} onDelete={handleDelete} />
        )}
        refreshControl={
          <RefreshControl refreshing={refreshing} onRefresh={handleRefresh} tintColor="#4f7cff" />
        }
        ListEmptyComponent={
          <View style={styles.empty}>
            <Text style={styles.emptyText}>No captures yet</Text>
            <Text style={styles.emptySub}>Type above to capture your first idea</Text>
          </View>
        }
        contentContainerStyle={captures.length === 0 ? styles.emptyContainer : undefined}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f1117',
    padding: 16,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    marginBottom: 16,
    marginTop: 8,
  },
  logo: {
    color: '#e4e6eb',
    fontSize: 24,
    fontWeight: '800',
  },
  subtitle: {
    color: '#6b7280',
    fontSize: 13,
    marginTop: 2,
  },
  quickCapture: {
    backgroundColor: '#1a1d27',
    borderRadius: 12,
    padding: 14,
    borderWidth: 1,
    borderColor: '#252836',
    marginBottom: 12,
  },
  input: {
    color: '#e4e6eb',
    fontSize: 15,
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
    color: '#6b7280',
    fontSize: 11,
  },
  captureBtn: {
    backgroundColor: '#4f7cff',
    borderRadius: 8,
    paddingHorizontal: 18,
    paddingVertical: 8,
  },
  captureBtnDisabled: {
    opacity: 0.4,
  },
  captureBtnText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 14,
  },
  toast: {
    backgroundColor: '#22c55e33',
    borderRadius: 8,
    padding: 10,
    marginBottom: 10,
    alignItems: 'center',
  },
  toastText: {
    color: '#22c55e',
    fontSize: 13,
    fontWeight: '500',
  },
  listHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 10,
  },
  listTitle: {
    color: '#e4e6eb',
    fontSize: 17,
    fontWeight: '700',
  },
  syncBtn: {
    backgroundColor: '#22c55e22',
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 6,
  },
  syncBtnActive: {
    opacity: 0.5,
  },
  syncBtnText: {
    color: '#22c55e',
    fontSize: 12,
    fontWeight: '600',
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
    color: '#6b7280',
    fontSize: 16,
    fontWeight: '600',
  },
  emptySub: {
    color: '#4b5563',
    fontSize: 13,
    marginTop: 6,
  },
});
