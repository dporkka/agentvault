import React, { useState, useCallback } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  Alert,
  ScrollView,
  StyleSheet,
} from 'react-native';
import { useFocusEffect } from '@react-navigation/native';
import { DEFAULT_BASE_URL } from '@agentvault/contract';
import { getSettings, saveSettings, clearInbox, getUnsyncedCaptures, markAsSynced } from '../storage/localInbox';
import { sendCapture } from '../api/agentvault';
import { checkHealth } from '../api/agentvault';
import type { AppSettings } from '../types';

export default function SettingsScreen() {
  const [settings, setSettingsState] = useState<AppSettings>({
    serverUrl: DEFAULT_BASE_URL,
    defaultProject: '',
    token: '',
  });
  const [health, setHealth] = useState<boolean | null>(null);
  const [syncing, setSyncing] = useState(false);

  const load = useCallback(async () => {
    const s = await getSettings();
    setSettingsState(s);
    const h = await checkHealth(s.serverUrl);
    setHealth(h);
  }, []);

  useFocusEffect(
    useCallback(() => {
      load();
    }, [load])
  );

  const update = (patch: Partial<AppSettings>) => {
    const next = { ...settings, ...patch };
    setSettingsState(next);
    saveSettings(next);
  };

  const handleTest = async () => {
    const ok = await checkHealth(settings.serverUrl);
    setHealth(ok);
    Alert.alert(ok ? 'Connected' : 'Unreachable', ok
      ? 'Server is responding.'
      : 'Could not reach the server. Check the URL and network.');
  };

  const handleSyncAll = async () => {
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
        break;
      }
    }
    setSyncing(false);
    Alert.alert('Sync Complete', sent > 0
      ? `Synced ${sent} of ${unsynced.length} captures.`
      : 'Nothing to sync or server unreachable.');
  };

  const handleClear = () => {
    Alert.alert('Clear Inbox', 'Delete all local captures? This cannot be undone.', [
      { text: 'Cancel', style: 'cancel' },
      {
        text: 'Clear',
        style: 'destructive',
        onPress: async () => {
          await clearInbox();
          Alert.alert('Done', 'Inbox cleared.');
        },
      },
    ]);
  };

  return (
    <ScrollView style={styles.container} contentContainerStyle={styles.content}>
      <Text style={styles.header}>Settings</Text>

      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Server</Text>
        <Text style={styles.label}>Server URL</Text>
        <TextInput
          style={styles.input}
          value={settings.serverUrl}
          onChangeText={(v) => update({ serverUrl: v })}
          placeholder={DEFAULT_BASE_URL}
          placeholderTextColor="#6b7280"
          autoCapitalize="none"
          keyboardType="url"
        />
        <View style={styles.healthRow}>
          <View style={[styles.healthDot, { backgroundColor: health === true ? '#22c55e' : health === false ? '#ef4444' : '#6b7280' }]} />
          <Text style={styles.healthText}>
            {health === true ? 'Online' : health === false ? 'Offline' : 'Unknown'}
          </Text>
          <TouchableOpacity style={styles.testBtn} onPress={handleTest}>
            <Text style={styles.testBtnText}>Test</Text>
          </TouchableOpacity>
        </View>
        <Text style={styles.label}>Auth Token</Text>
        <TextInput
          style={styles.input}
          value={settings.token}
          onChangeText={(v) => update({ token: v })}
          placeholder="X-AgentVault-Token (printed by 'serve')"
          placeholderTextColor="#6b7280"
          autoCapitalize="none"
          autoCorrect={false}
          secureTextEntry
        />
        <Text style={styles.hint}>Required to sync captures to the server.</Text>
      </View>

      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Defaults</Text>
        <Text style={styles.label}>Default Project</Text>
        <TextInput
          style={styles.input}
          value={settings.defaultProject}
          onChangeText={(v) => update({ defaultProject: v })}
          placeholder="e.g. inbox"
          placeholderTextColor="#6b7280"
          autoCapitalize="none"
        />
      </View>

      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Actions</Text>

        <TouchableOpacity
          style={[styles.actionBtn, styles.actionBtnPrimary]}
          onPress={handleSyncAll}
          disabled={syncing}
        >
          <Text style={styles.actionBtnTextPrimary}>
            {syncing ? 'Syncing...' : 'Sync All to Server'}
          </Text>
        </TouchableOpacity>

        <TouchableOpacity
          style={[styles.actionBtn, styles.actionBtnDanger]}
          onPress={handleClear}
        >
          <Text style={styles.actionBtnTextDanger}>Clear Local Inbox</Text>
        </TouchableOpacity>
      </View>

      <View style={styles.section}>
        <Text style={styles.sectionTitle}>About</Text>
        <View style={styles.infoRow}>
          <Text style={styles.infoLabel}>App</Text>
          <Text style={styles.infoValue}>AgentVault Mobile</Text>
        </View>
        <View style={styles.infoRow}>
          <Text style={styles.infoLabel}>Version</Text>
          <Text style={styles.infoValue}>0.1.0</Text>
        </View>
        <Text style={styles.aboutText}>
          Capture-first mobile companion for AgentVault. All captures are stored
          locally and synced when the server is reachable.
        </Text>
      </View>
    </ScrollView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f1117',
  },
  content: {
    padding: 16,
    paddingBottom: 40,
  },
  header: {
    color: '#e4e6eb',
    fontSize: 22,
    fontWeight: '800',
    marginBottom: 20,
    marginTop: 8,
  },
  section: {
    marginBottom: 24,
  },
  sectionTitle: {
    color: '#4f7cff',
    fontSize: 13,
    fontWeight: '700',
    marginBottom: 10,
    textTransform: 'uppercase',
    letterSpacing: 0.8,
  },
  label: {
    color: '#9ca3af',
    fontSize: 12,
    fontWeight: '600',
    marginBottom: 6,
  },
  hint: {
    color: '#6b7280',
    fontSize: 11,
    marginTop: -4,
    marginBottom: 4,
  },
  input: {
    backgroundColor: '#1a1d27',
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 12,
    color: '#e4e6eb',
    fontSize: 15,
    borderWidth: 1,
    borderColor: '#252836',
    marginBottom: 10,
  },
  healthRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 8,
  },
  healthDot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  healthText: {
    color: '#9ca3af',
    fontSize: 13,
    flex: 1,
  },
  testBtn: {
    backgroundColor: '#252836',
    borderRadius: 6,
    paddingHorizontal: 14,
    paddingVertical: 6,
  },
  testBtnText: {
    color: '#4f7cff',
    fontSize: 13,
    fontWeight: '600',
  },
  actionBtn: {
    borderRadius: 10,
    paddingVertical: 14,
    alignItems: 'center',
    marginBottom: 10,
  },
  actionBtnPrimary: {
    backgroundColor: '#4f7cff',
  },
  actionBtnDanger: {
    backgroundColor: '#ef444422',
    borderWidth: 1,
    borderColor: '#ef4444',
  },
  actionBtnTextPrimary: {
    color: '#fff',
    fontSize: 15,
    fontWeight: '700',
  },
  actionBtnTextDanger: {
    color: '#ef4444',
    fontSize: 15,
    fontWeight: '700',
  },
  infoRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 8,
    borderBottomWidth: 1,
    borderBottomColor: '#1a1d27',
  },
  infoLabel: {
    color: '#6b7280',
    fontSize: 14,
  },
  infoValue: {
    color: '#e4e6eb',
    fontSize: 14,
    fontWeight: '500',
  },
  aboutText: {
    color: '#6b7280',
    fontSize: 13,
    lineHeight: 18,
    marginTop: 12,
  },
});
