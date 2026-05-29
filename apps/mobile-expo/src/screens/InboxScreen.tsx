import React, { useState, useCallback } from 'react';
import {
  View,
  Text,
  FlatList,
  RefreshControl,
  TouchableOpacity,
  Alert,
  StyleSheet,
} from 'react-native';
import { useFocusEffect } from '@react-navigation/native';
import type { Capture } from '../types';
import {
  getCaptures,
  getUnsyncedCaptures,
  markAsSynced,
  deleteCapture,
} from '../storage/localInbox';
import { sendCapture } from '../api/agentvault';
import CaptureCard from '../components/CaptureCard';
import ConnectionBadge from '../components/ConnectionBadge';

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
  const [grouped, setGrouped] = useState<GroupedCaptures[]>([]);
  const [refreshing, setRefreshing] = useState(false);
  const [syncing, setSyncing] = useState(false);

  const load = useCallback(async () => {
    const list = await getCaptures();
    setGrouped(groupByDate(list));
  }, []);

  useFocusEffect(
    useCallback(() => {
      load();
    }, [load])
  );

  const handleSync = async () => {
    setRefreshing(true);
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
        break;
      }
    }
    await load();
    setRefreshing(false);
  };

  const handleDelete = (id: string) => {
    Alert.alert('Delete Capture', 'Are you sure?', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Delete',
        style: 'destructive',
        onPress: async () => {
          await deleteCapture(id);
          load();
        },
      },
    ]);
  };

  const handleSyncOne = async (cap: Capture) => {
    if (cap.synced) return;
    try {
      await sendCapture({
        type: cap.type,
        title: cap.title,
        text: cap.text,
        project: cap.project,
        tags: cap.tags,
      });
      await markAsSynced(cap.id);
      load();
    } catch {
      Alert.alert('Error', 'Could not sync. Server unreachable?');
    }
  };

  return (
    <View style={styles.container}>
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
          <RefreshControl
            refreshing={refreshing}
            onRefresh={handleSync}
            tintColor="#4f7cff"
          />
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
            <Text style={styles.emptySub}>
              Captures are saved here for offline access
            </Text>
          </View>
        }
        contentContainerStyle={
          grouped.length === 0 ? styles.emptyContainer : undefined
        }
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
    marginBottom: 12,
    marginTop: 8,
  },
  title: {
    color: '#e4e6eb',
    fontSize: 22,
    fontWeight: '800',
  },
  subtitle: {
    color: '#6b7280',
    fontSize: 13,
    marginTop: 2,
  },
  group: {
    marginBottom: 8,
  },
  groupDate: {
    color: '#6b7280',
    fontSize: 13,
    fontWeight: '600',
    marginBottom: 8,
    marginTop: 4,
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
    color: '#6b7280',
    fontSize: 16,
    fontWeight: '600',
  },
  emptySub: {
    color: '#4b5563',
    fontSize: 13,
    marginTop: 6,
    textAlign: 'center',
    paddingHorizontal: 40,
  },
});
