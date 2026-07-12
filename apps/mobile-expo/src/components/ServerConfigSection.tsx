import React from 'react';
import { View, Text, TextInput, TouchableOpacity, StyleSheet } from 'react-native';
import { DEFAULT_BASE_URL } from '@agentvault/contract';
import { colors } from '../theme';
import type { AppSettings } from '../types';

interface Props {
  draft: AppSettings;
  health: boolean | null;
  tokenStatus: 'unknown' | 'missing' | 'invalid' | 'valid';
  verifying: boolean;
  onUpdate: (patch: Partial<AppSettings>) => void;
  onTest: () => void;
  onVerifyToken: () => void;
}

export default function ServerConfigSection({
  draft,
  health,
  tokenStatus,
  verifying,
  onUpdate,
  onTest,
  onVerifyToken,
}: Props) {
  return (
    <View style={styles.section}>
      <Text style={styles.sectionTitle}>Server</Text>
      <Text style={styles.label}>Server URL</Text>
      <TextInput
        style={styles.input}
        value={draft.serverUrl}
        onChangeText={(v) => onUpdate({ serverUrl: v })}
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
                health === true ? colors.success : health === false ? colors.error : colors.textMuted,
            },
          ]}
        />
        <Text style={styles.healthText}>
          {health === true ? 'Online' : health === false ? 'Offline' : 'Unknown'}
        </Text>
        <TouchableOpacity
          style={styles.testBtn}
          onPress={onTest}
          accessibilityRole="button"
          accessibilityLabel="Test server connection"
        >
          <Text style={styles.testBtnText}>Test</Text>
        </TouchableOpacity>
      </View>
      <Text style={styles.label}>
        Auth Token
        {tokenStatus === 'valid' && <Text style={{ color: colors.success }}> • valid</Text>}
        {tokenStatus === 'invalid' && <Text style={{ color: colors.error }}> • invalid</Text>}
        {tokenStatus === 'missing' && <Text style={{ color: colors.warning }}> • missing</Text>}
      </Text>
      <TextInput
        style={styles.input}
        value={draft.token}
        onChangeText={(v) => onUpdate({ token: v })}
        placeholder="X-AgentVault-Token (printed by 'serve')"
        placeholderTextColor={colors.textMuted}
        autoCapitalize="none"
        autoCorrect={false}
        secureTextEntry
      />
      <TouchableOpacity
        style={[styles.actionBtn, styles.actionBtnSecondary]}
        onPress={onVerifyToken}
        disabled={verifying}
        accessibilityRole="button"
        accessibilityLabel={verifying ? 'Verifying token' : 'Verify auth token'}
        accessibilityState={{ disabled: verifying }}
      >
        <Text style={styles.actionBtnTextSecondary}>
          {verifying ? 'Verifying...' : 'Verify Token'}
        </Text>
      </TouchableOpacity>
      <Text style={styles.hint}>Required to sync captures to the server.</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  section: { marginBottom: 28 },
  sectionTitle: { fontSize: 15, fontWeight: '600', color: colors.textPrimary, marginBottom: 14 },
  label: { fontSize: 12, color: colors.textSecondary, marginBottom: 6 },
  input: {
    backgroundColor: colors.bgTertiary,
    borderColor: colors.border,
    borderWidth: 1,
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    fontSize: 13,
    color: colors.textPrimary,
    marginBottom: 12,
  },
  healthRow: { flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 12 },
  healthDot: { width: 8, height: 8, borderRadius: 4 },
  healthText: { fontSize: 12, color: colors.textSecondary },
  testBtn: {
    marginLeft: 'auto',
    paddingHorizontal: 12,
    paddingVertical: 6,
    borderRadius: 6,
    backgroundColor: colors.bgTertiary,
  },
  testBtnText: { fontSize: 12, color: colors.accent },
  actionBtn: {
    paddingVertical: 12,
    borderRadius: 8,
    alignItems: 'center',
    marginBottom: 8,
  },
  actionBtnSecondary: { backgroundColor: colors.bgTertiary },
  actionBtnTextSecondary: { fontSize: 13, fontWeight: '500', color: colors.accent },
  hint: { fontSize: 11, color: colors.textMuted, marginTop: 4 },
});
