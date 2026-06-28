import React, { useState, useEffect } from 'react';
import { View, Text, TextInput, TouchableOpacity, Modal, FlatList, StyleSheet } from 'react-native';
import Ionicons from '@expo/vector-icons/Ionicons';
import { getProjects } from '../api/agentvault';
import { useSettings } from '../context/SettingsContext';
import { colors, spacing, radii, typography, layout } from '../theme';

interface ProjectPickerProps {
  selected: string;
  onChange: (project: string) => void;
}

const DEFAULT_PROJECTS = ['personal', 'work', 'inbox', 'learning'];

export default function ProjectPicker({ selected, onChange }: ProjectPickerProps) {
  const { settings } = useSettings();
  const [open, setOpen] = useState(false);
  const [projects, setProjects] = useState<string[]>(DEFAULT_PROJECTS);
  const [customInput, setCustomInput] = useState('');

  useEffect(() => {
    let mounted = true;
    getProjects(settings.serverUrl)
      .then((list) => {
        if (mounted && list.length > 0) {
          setProjects((prev) => [...new Set([...prev, ...list])]);
        }
      })
      .catch(() => {
        // silently fall back to defaults
      });
    return () => {
      mounted = false;
    };
  }, [settings.serverUrl]);

  const selectProject = (name: string) => {
    onChange(name);
    setOpen(false);
  };

  const addCustom = () => {
    const trimmed = customInput.trim().toLowerCase();
    if (trimmed) {
      if (!projects.includes(trimmed)) {
        setProjects([...projects, trimmed]);
      }
      onChange(trimmed);
      setCustomInput('');
      setOpen(false);
    }
  };

  return (
    <>
      <TouchableOpacity style={styles.selector} onPress={() => setOpen(true)}>
        <Text style={styles.selectorLabel}>Project</Text>
        <Text style={styles.selectorValue}>{selected || 'Select project...'}</Text>
      </TouchableOpacity>

      <Modal visible={open} transparent animationType="slide" onRequestClose={() => setOpen(false)}>
        <View style={styles.overlay}>
          <View style={styles.sheet}>
            <View style={styles.sheetHeader}>
              <Text style={styles.sheetTitle}>Select Project</Text>
              <TouchableOpacity onPress={() => setOpen(false)}>
                <Text style={styles.closeBtn}>Close</Text>
              </TouchableOpacity>
            </View>

            <View style={styles.customRow}>
              <TextInput
                style={styles.customInput}
                placeholder="New project..."
                placeholderTextColor={colors.textMuted}
                value={customInput}
                onChangeText={setCustomInput}
                onSubmitEditing={addCustom}
              />
              <TouchableOpacity style={styles.customAddBtn} onPress={addCustom}>
                <Text style={styles.customAddText}>Add</Text>
              </TouchableOpacity>
            </View>

            <FlatList
              data={projects}
              keyExtractor={(item) => item}
              renderItem={({ item }) => (
                <TouchableOpacity
                  style={[styles.projectItem, item === selected && styles.projectItemActive]}
                  onPress={() => selectProject(item)}
                >
                  <Text
                    style={[
                      styles.projectItemText,
                      item === selected && styles.projectItemTextActive,
                    ]}
                  >
                    {item}
                  </Text>
                  {item === selected && (
                    <Ionicons name="checkmark" size={18} color={colors.accent} />
                  )}
                </TouchableOpacity>
              )}
            />
          </View>
        </View>
      </Modal>
    </>
  );
}

const styles = StyleSheet.create({
  selector: {
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.md,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
    marginVertical: spacing.sm,
  },
  selectorLabel: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.medium,
    marginBottom: 2,
  },
  selectorValue: {
    color: colors.textPrimary,
    fontSize: typography.sizes.base,
    fontWeight: typography.weights.medium,
  },
  overlay: {
    flex: 1,
    backgroundColor: '#00000088',
    justifyContent: 'flex-end',
  },
  sheet: {
    backgroundColor: colors.bgPrimary,
    borderTopLeftRadius: radii.xxl,
    borderTopRightRadius: radii.xxl,
    paddingHorizontal: spacing.xl,
    paddingTop: spacing.lg,
    paddingBottom: 40,
    maxHeight: layout.maxSheetHeight,
  },
  sheetHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: spacing.lg,
  },
  sheetTitle: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xl,
    fontWeight: typography.weights.bold,
  },
  closeBtn: {
    color: colors.accent,
    fontSize: typography.sizes.base,
    fontWeight: typography.weights.semibold,
  },
  customRow: {
    flexDirection: 'row',
    gap: spacing.sm,
    marginBottom: spacing.md,
  },
  customInput: {
    flex: 1,
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md,
    paddingVertical: 10,
    color: colors.textPrimary,
    fontSize: typography.sizes.base,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
  },
  customAddBtn: {
    backgroundColor: colors.accent,
    borderRadius: radii.md,
    paddingHorizontal: 16,
    justifyContent: 'center',
  },
  customAddText: {
    color: '#fff',
    fontWeight: typography.weights.semibold,
    fontSize: typography.sizes.md,
  },
  projectItem: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 14,
    paddingHorizontal: spacing.md,
    borderRadius: radii.md,
  },
  projectItemActive: {
    backgroundColor: `${colors.accent}22`,
  },
  projectItemText: {
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
  },
  projectItemTextActive: {
    color: colors.accent,
    fontWeight: typography.weights.semibold,
  },
});
