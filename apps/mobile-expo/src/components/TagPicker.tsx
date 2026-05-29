import React, { useState } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  ScrollView,
  StyleSheet,
} from 'react-native';

interface TagPickerProps {
  selected: string[];
  onChange: (tags: string[]) => void;
  suggestions?: string[];
}

const DEFAULT_TAGS = [
  'idea',
  'todo',
  'note',
  'meeting',
  'research',
  'bug',
  'feature',
  'refactor',
];

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
          placeholderTextColor="#6b7280"
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
            <TouchableOpacity
              key={tag}
              style={styles.selectedTag}
              onPress={() => toggleTag(tag)}
            >
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
              <Text style={[styles.chipText, isActive && styles.chipTextActive]}>
                {tag}
              </Text>
            </TouchableOpacity>
          );
        })}
      </ScrollView>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    marginVertical: 8,
  },
  label: {
    color: '#e4e6eb',
    fontSize: 14,
    fontWeight: '600',
    marginBottom: 6,
  },
  inputRow: {
    flexDirection: 'row',
    gap: 8,
    marginBottom: 8,
  },
  input: {
    flex: 1,
    backgroundColor: '#1a1d27',
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 10,
    color: '#e4e6eb',
    fontSize: 14,
    borderWidth: 1,
    borderColor: '#252836',
  },
  addBtn: {
    backgroundColor: '#4f7cff',
    borderRadius: 8,
    width: 42,
    alignItems: 'center',
    justifyContent: 'center',
  },
  addBtnText: {
    color: '#fff',
    fontSize: 18,
    fontWeight: '600',
  },
  selectedRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 6,
    marginBottom: 10,
  },
  selectedTag: {
    backgroundColor: '#4f7cff22',
    borderColor: '#4f7cff',
    borderWidth: 1,
    borderRadius: 8,
    paddingHorizontal: 10,
    paddingVertical: 5,
  },
  selectedTagText: {
    color: '#4f7cff',
    fontSize: 12,
    fontWeight: '500',
  },
  suggestions: {
    maxHeight: 44,
  },
  suggestionsContent: {
    gap: 8,
    paddingRight: 10,
  },
  chip: {
    backgroundColor: '#252836',
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 6,
    marginRight: 6,
  },
  chipActive: {
    backgroundColor: '#4f7cff33',
    borderColor: '#4f7cff',
    borderWidth: 1,
  },
  chipText: {
    color: '#9ca3af',
    fontSize: 12,
  },
  chipTextActive: {
    color: '#4f7cff',
    fontWeight: '600',
  },
});
