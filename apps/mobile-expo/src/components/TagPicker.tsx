import React, { useState } from 'react';
import { View, Text, TextInput, TouchableOpacity, ScrollView, StyleSheet } from 'react-native';
import { colors, spacing, radii, typography } from '../theme';

interface TagPickerProps {
  selected: string[];
  onChange: (tags: string[]) => void;
  suggestions?: string[];
}

const DEFAULT_TAGS = ['idea', 'todo', 'note', 'meeting', 'research', 'bug', 'feature', 'refactor'];

export default function TagPicker({ selected, onChange, suggestions }: TagPickerProps) {
  const [input, setInput] = useState('');
  const allSuggestions = [...new Set([...DEFAULT_TAGS, ...(suggestions || [])])];

  const toggleTag = (tag: string) => {
    if (selected.includes(tag)) {
      onChange(selected.filter((t) => t !== tag));
    } else {
      onChange([...selected, tag]);
    }
  };

  const addCustomTag = () => {
    const trimmed = input.trim().toLowerCase();
    if (trimmed && !selected.includes(trimmed)) {
      onChange([...selected, trimmed]);
      setInput('');
    }
  };

  return (
    <View style={styles.container}>
      <Text style={styles.label}>Tags</Text>

      <View style={styles.inputRow}>
        <TextInput
          style={styles.input}
          placeholder="Add tag..."
          placeholderTextColor={colors.textMuted}
          value={input}
          onChangeText={setInput}
          onSubmitEditing={addCustomTag}
          returnKeyType="done"
        />
        <TouchableOpacity style={styles.addBtn} onPress={addCustomTag}>
          <Text style={styles.addBtnText}>+</Text>
        </TouchableOpacity>
      </View>

      {selected.length > 0 && (
        <View style={styles.selectedRow}>
          {selected.map((tag) => (
            <TouchableOpacity key={tag} style={styles.selectedTag} onPress={() => toggleTag(tag)}>
              <Text style={styles.selectedTagText}>{tag} x</Text>
            </TouchableOpacity>
          ))}
        </View>
      )}

      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        style={styles.suggestions}
        contentContainerStyle={styles.suggestionsContent}
      >
        {allSuggestions.map((tag) => {
          const isActive = selected.includes(tag);
          return (
            <TouchableOpacity
              key={tag}
              style={[styles.chip, isActive && styles.chipActive]}
              onPress={() => toggleTag(tag)}
            >
              <Text style={[styles.chipText, isActive && styles.chipTextActive]}>{tag}</Text>
            </TouchableOpacity>
          );
        })}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    marginVertical: spacing.sm,
  },
  label: {
    color: colors.textPrimary,
    fontSize: typography.sizes.base,
    fontWeight: typography.weights.semibold,
    marginBottom: 6,
  },
  inputRow: {
    flexDirection: 'row',
    gap: spacing.sm,
    marginBottom: spacing.sm,
  },
  input: {
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
  addBtn: {
    backgroundColor: colors.accent,
    borderRadius: radii.md,
    width: 42,
    alignItems: 'center',
    justifyContent: 'center',
  },
  addBtnText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: typography.weights.semibold,
  },
  selectedRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 6,
    marginBottom: 10,
  },
  selectedTag: {
    backgroundColor: `${colors.accent}22`,
    borderColor: colors.accent,
    borderWidth: 1,
    borderRadius: radii.md,
    paddingHorizontal: 10,
    paddingVertical: 5,
  },
  selectedTagText: {
    color: colors.accent,
    fontSize: typography.sizes.sm,
    fontWeight: typography.weights.medium,
  },
  suggestions: {
    maxHeight: 44,
  },
  suggestionsContent: {
    gap: spacing.sm,
    paddingRight: 10,
  },
  chip: {
    backgroundColor: colors.borderSubtle,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md,
    paddingVertical: 6,
    marginRight: 6,
  },
  chipActive: {
    backgroundColor: colors.accentMuted,
    borderColor: colors.accent,
    borderWidth: 1,
  },
  chipText: {
    color: colors.textSecondary,
    fontSize: typography.sizes.sm,
  },
  chipTextActive: {
    color: colors.accent,
    fontWeight: typography.weights.semibold,
  },
});
