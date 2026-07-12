import React from 'react';
import { View, Text, TextInput, TouchableOpacity, StyleSheet } from 'react-native';
import { colors } from '../theme';
import type { AppSettings } from '../types';

interface Props {
  draft: AppSettings;
  syncing: boolean;
  onUpdate: (patch: Partial<AppSettings>) => void;
  onSyncAll: () => void;
  onClear: () => void;
}

export default function ActionsSection({ draft, syncing, onUpdate, onSyncAll, onClear }: Props) {
  return (
    <>
      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Defaults</Text>
        <Text style={styles.label}>Default Project</Text>
        <TextInput
          style={styles.input}
          value={draft.defaultProject}
          onChangeText={(v) => onUpdate({ defaultProject: v })}
          placeholder="e.g. inbox"
          placeholderTextColor={colors.textMuted}
          autoCapitalize="none"
        />
      </View>

      <View style={styles.section}>
        <Text style={styles.sectionTitle}>Actions</Text>
        <TouchableOpacity
          style={[styles.actionBtn, styles.actionBtnPrimary]}
          onPress={onSyncAll}
          disabled={syncing}
          accessibilityRole="button"
          accessibilityLabel={syncing ? 'Syncing all captures' : 'Sync all captures to server'}
          accessibilityState={{ disabled: syncing }}
        >
          <Text style={styles.actionBtnTextPrimary}>
            {syncing ? 'Syncing...' : 'Sync All to Server'}
          </Text>
        </TouchableOpacity>
        <TouchableOpacity
          style={[styles.actionBtn, styles.actionBtnDanger]}
          onPress={onClear}
          accessibilityRole="button"
          accessibilityLabel="Clear local inbox"
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
          Capture-first mobile companion for AgentVault. All captures are stored locally and synced
          when the server is reachable.
        </Text>
      </View>
    </>
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
  actionBtn: {
    paddingVertical: 12,
    borderRadius: 8,
    alignItems: 'center',
    marginBottom: 8,
  },
  actionBtnPrimary: { backgroundColor: colors.accent },
  actionBtnTextPrimary: { fontSize: 13, fontWeight: '600', color: colors.textInverse },
  actionBtnDanger: { backgroundColor: colors.errorMuted },
  actionBtnTextDanger: { fontSize: 13, fontWeight: '600', color: colors.error },
  infoRow: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    paddingVertical: 8,
    borderBottomWidth: StyleSheet.hairlineWidth,
    borderBottomColor: colors.borderSubtle,
  },
  infoLabel: { fontSize: 13, color: colors.textSecondary },
  infoValue: { fontSize: 13, color: colors.textPrimary },
  aboutText: { fontSize: 12, color: colors.textMuted, marginTop: 12, lineHeight: 18 },
});
