import React, { useState, useEffect, useCallback, useRef } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  Alert,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
  StyleSheet,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { DEFAULT_BASE_URL } from '@agentvault/contract';
import { clearInbox } from '../storage/localInbox';
import { useSettings } from '../context/SettingsContext';
import { syncCaptures, formatSyncResult } from '../storage/sync';
import { checkHealth, verifyToken } from '../api/agentvault';
import { DEFAULT_BASE_URL } from '@agentvault/contract';
import { clearInbox, getUnsyncedCaptures, markAsSynced } from '../storage/localInbox';
import { useSettings } from '../context/SettingsContext';
import { sendCapture, checkHealth, verifyToken } from '../api/agentvault';
import type { AppSettings } from '../types';
import { colors, spacing, radii, typography } from '../theme';

export default function SettingsScreen() {
  const { settings, saveSettings, loaded } = useSettings();
  const [draft, setDraft] = useState<AppSettings>(settings);
  const [health, setHealth] = useState<boolean | null>(null);
  const [tokenStatus, setTokenStatus] = useState<'unknown' | 'missing' | 'invalid' | 'valid'>(
    'unknown',
  );
  const [verifying, setVerifying] = useState(false);
  const [syncing, setSyncing] = useState(false);
  const saveTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (saveTimeoutRef.current) clearTimeout(saveTimeoutRef.current);
    };
  }, []);

  const scheduleSave = useCallback(
    (next: AppSettings) => {
      if (saveTimeoutRef.current) clearTimeout(saveTimeoutRef.current);
      saveTimeoutRef.current = setTimeout(() => {
        saveSettings(next);
      }, 400);
    },
    [saveSettings],
  );

  const update = (patch: Partial<AppSettings>) => {
    const next = { ...draft, ...patch };
    setDraft(next);
    scheduleSave(next);
  };

  const handleTest = async () => {
    // Persist the URL being tested so the client config matches.
    await saveSettings(draft);
    const ok = await checkHealth(draft.serverUrl);
    setHealth(ok);
    Alert.alert(
      ok ? 'Connected' : 'Unreachable',
      ok ? 'Server is responding.' : 'Could not reach the server. Check the URL and network.',
    );
  };

  const handleVerifyToken = async () => {
    await saveSettings(draft);
    setVerifying(true);
    const result = await verifyToken(draft.serverUrl);
    setVerifying(false);
    if (!result) {
      setTokenStatus('unknown');
      Alert.alert('Unreachable', 'Could not contact the server to verify the token.');
      return;
    }
    if (!result.hasToken) {
      setTokenStatus('missing');
      Alert.alert(
        'Token Missing',
        'No token was sent. Enter the token printed by agentvault serve.',
      );
      return;
    }
    if (!result.tokenValid) {
      setTokenStatus('invalid');
      Alert.alert(
        'Invalid Token',
        'The server rejected this token. Copy the token printed by agentvault serve.',
      );
      return;
    }
    setTokenStatus('valid');
    Alert.alert('Token Valid', 'Your token is accepted by the server.');
  };

  const handleSyncAll = async () => {
    setSyncing(true);
    const result = await syncCaptures({ continueOnError: true });
    setSyncing(false);
    Alert.alert('Sync Complete', formatSyncResult(result));
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

  if (!loaded) {
    return (
      <SafeAreaView style={[styles.container, styles.centered]} edges={['top', 'left', 'right']}>
        <Text style={styles.loadingText}>Loading settings...</Text>
      </SafeAreaView>
    );
  }

  return (
    <SafeAreaView style={styles.container} edges={['top', 'left', 'right']}>
      <KeyboardAvoidingView
        style={styles.keyboard}
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        keyboardVerticalOffset={80}
      >
        <ScrollView style={styles.scroll} contentContainerStyle={styles.content}>
          <Text style={styles.header}>Settings</Text>

          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Server</Text>
            <Text style={styles.label}>Server URL</Text>
            <TextInput
              style={styles.input}
              value={draft.serverUrl}
              onChangeText={(v) => update({ serverUrl: v })}
              placeholder={DEFAULT_BASE_URL}
              placeholderTextColor={colors.textMuted}
              autoCapitalize="none"
              keyboardType="url"
            />
            <View style={styles.healthRow}>
              <View
                style={[
                  styles.healthDot,
                  {
                    backgroundColor:
                      health === true
                        ? colors.success
                        : health === false
                          ? colors.error
                          : colors.textMuted,
                  },
                ]}
              />
              <Text style={styles.healthText}>
                {health === true ? 'Online' : health === false ? 'Offline' : 'Unknown'}
              </Text>
              <TouchableOpacity style={styles.testBtn} onPress={handleTest}>
                <Text style={styles.testBtnText}>Test</Text>
              </TouchableOpacity>
            </View>
            <Text style={styles.label}>
              Auth Token
              {tokenStatus === 'valid' && <Text style={{ color: colors.success }}> • valid</Text>}
              {tokenStatus === 'invalid' && <Text style={{ color: colors.error }}> • invalid</Text>}
              {tokenStatus === 'missing' && (
                <Text style={{ color: colors.warning }}> • missing</Text>
              )}
            </Text>
            <TextInput
              style={styles.input}
              value={draft.token}
              onChangeText={(v) => update({ token: v })}
              placeholder="X-AgentVault-Token (printed by 'serve')"
              placeholderTextColor={colors.textMuted}
              autoCapitalize="none"
              autoCorrect={false}
              secureTextEntry
            />
            <TouchableOpacity
              style={[styles.actionBtn, styles.actionBtnSecondary]}
              onPress={handleVerifyToken}
              disabled={verifying}
            >
              <Text style={styles.actionBtnTextSecondary}>
                {verifying ? 'Verifying...' : 'Verify Token'}
              </Text>
            </TouchableOpacity>
            <Text style={styles.hint}>Required to sync captures to the server.</Text>
          </View>

          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Defaults</Text>
            <Text style={styles.label}>Default Project</Text>
            <TextInput
              style={styles.input}
              value={draft.defaultProject}
              onChangeText={(v) => update({ defaultProject: v })}
              placeholder="e.g. inbox"
              placeholderTextColor={colors.textMuted}
              autoCapitalize="none"
            />
          </View>
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Server</Text>
        <Text style={styles.label}>Server URL</Text>
        <TextInput
          style={styles.input}
          value={draft.serverUrl}
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
        <Text style={styles.label}>
          Auth Token
          {tokenStatus === 'valid' && <Text style={{ color: '#22c55e' }}> • valid</Text>}
          {tokenStatus === 'invalid' && <Text style={{ color: '#ef4444' }}> • invalid</Text>}
          {tokenStatus === 'missing' && <Text style={{ color: '#f59e0b' }}> • missing</Text>}
        </Text>
        <TextInput
          style={styles.input}
          value={draft.token}
          onChangeText={(v) => update({ token: v })}
          placeholder="X-AgentVault-Token (printed by 'serve')"
          placeholderTextColor="#6b7280"
          autoCapitalize="none"
          autoCorrect={false}
          secureTextEntry
        />
        <TouchableOpacity
          style={[styles.actionBtn, styles.actionBtnSecondary]}
          onPress={handleVerifyToken}
          disabled={verifying}
        >
          <Text style={styles.actionBtnTextSecondary}>
            {verifying ? 'Verifying...' : 'Verify Token'}
          </Text>
        </TouchableOpacity>
        <Text style={styles.hint}>Required to sync captures to the server.</Text>
      </View>

      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Defaults</Text>
        <Text style={styles.label}>Default Project</Text>
        <TextInput
          style={styles.input}
          value={draft.defaultProject}
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
              Capture-first mobile companion for AgentVault. All captures are stored locally and
              synced when the server is reachable.
            </Text>
          </View>
        </ScrollView>
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.bgPrimary,
  },
  keyboard: {
    flex: 1,
  },
  scroll: {
    flex: 1,
  },
  content: {
    padding: spacing.lg,
    paddingBottom: 40,
  },
  header: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xxl,
    fontWeight: typography.weights.extrabold,
    marginBottom: spacing.xl,
    marginTop: spacing.sm,
  },
  section: {
    marginBottom: spacing.xxl,
  },
  sectionTitle: {
    color: colors.accent,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.bold,
    marginBottom: 10,
    textTransform: 'uppercase',
    letterSpacing: 0.8,
  },
  label: {
    color: colors.textSecondary,
    fontSize: typography.sizes.sm,
    fontWeight: typography.weights.semibold,
    marginBottom: 6,
  },
  hint: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
    marginTop: -4,
    marginBottom: spacing.xs,
  },
  input: {
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.md,
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
    marginBottom: 10,
  },
  healthRow: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
  },
  healthDot: {
    width: spacing.sm,
    height: spacing.sm,
    borderRadius: 4,
  },
  healthText: {
    color: colors.textSecondary,
    fontSize: typography.sizes.md,
    flex: 1,
  },
  testBtn: {
    backgroundColor: colors.borderSubtle,
    borderRadius: radii.sm,
    paddingHorizontal: 14,
    paddingVertical: 6,
  },
  testBtnText: {
    color: colors.accent,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.semibold,
  },
  actionBtn: {
    borderRadius: radii.lg,
    paddingVertical: 14,
    alignItems: 'center',
    marginBottom: 10,
  },
  actionBtnPrimary: {
    backgroundColor: colors.accent,
  },
  actionBtnSecondary: {
    backgroundColor: colors.bgSecondary,
    borderWidth: 1,
    borderColor: colors.accent,
  },
  actionBtnDanger: {
    backgroundColor: colors.errorMuted,
    borderWidth: 1,
    borderColor: colors.error,
  },
  actionBtnTextPrimary: {
    color: '#fff',
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.bold,
  },
  actionBtnTextSecondary: {
    color: colors.accent,
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.bold,
  },
  actionBtnTextDanger: {
    color: colors.error,
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.bold,
  },
  infoRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: spacing.sm,
    borderBottomWidth: 1,
    borderBottomColor: colors.bgSecondary,
  },
  infoLabel: {
    color: colors.textMuted,
    fontSize: typography.sizes.base,
  },
  infoValue: {
    color: colors.textPrimary,
    fontSize: typography.sizes.base,
    fontWeight: typography.weights.medium,
  },
  aboutText: {
    color: colors.textMuted,
    fontSize: typography.sizes.md,
    lineHeight: 18,
    marginTop: spacing.md,
  },
  centered: {
    alignItems: 'center',
    justifyContent: 'center',
  },
  loadingText: {
    color: colors.textMuted,
    fontSize: typography.sizes.lg,
  },
});
