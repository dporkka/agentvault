import React, { useState, useEffect } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  Modal,
  FlatList,
  StyleSheet,
} from 'react-native';
import { getProjects } from '../api/agentvault';

interface ProjectPickerProps {
  selected: string;
  onChange: (project: string) => void;
}

const DEFAULT_PROJECTS = ['personal', 'work', 'inbox', 'learning'];

export default function ProjectPicker({ selected, onChange }: ProjectPickerProps) {
  const [open, setOpen] = useState(false);
  const [projects, setProjects] = useState<string[]>(DEFAULT_PROJECTS);
  const [customInput, setCustomInput] = useState('');

  useEffect(() => {
    let mounted = true;
    getProjects()
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
  }, []);

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
        <Text style={styles.selectorValue}>
          {selected || 'Select project...'}
        </Text>
      </TouchableOpacity>

      <Modal
        visible={open}
        transparent
        animationType="slide"
        onRequestClose={() => setOpen(false)}
      >
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
                placeholderTextColor="#6b7280"
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
                  style={[
                    styles.projectItem,
                    item === selected && styles.projectItemActive,
                  ]}
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
                    <Text style={styles.checkmark}>x</Text>
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
    backgroundColor: '#1a1d27',
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 12,
    borderWidth: 1,
    borderColor: '#252836',
    marginVertical: 8,
  },
  selectorLabel: {
    color: '#6b7280',
    fontSize: 11,
    fontWeight: '500',
    marginBottom: 2,
  },
  selectorValue: {
    color: '#e4e6eb',
    fontSize: 14,
    fontWeight: '500',
  },
  overlay: {
    flex: 1,
    backgroundColor: '#00000088',
    justifyContent: 'flex-end',
  },
  sheet: {
    backgroundColor: '#0f1117',
    borderTopLeftRadius: 20,
    borderTopRightRadius: 20,
    paddingHorizontal: 20,
    paddingTop: 16,
    paddingBottom: 40,
    maxHeight: '60%',
  },
  sheetHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 16,
  },
  sheetTitle: {
    color: '#e4e6eb',
    fontSize: 17,
    fontWeight: '700',
  },
  closeBtn: {
    color: '#4f7cff',
    fontSize: 14,
    fontWeight: '600',
  },
  customRow: {
    flexDirection: 'row',
    gap: 8,
    marginBottom: 12,
  },
  customInput: {
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
  customAddBtn: {
    backgroundColor: '#4f7cff',
    borderRadius: 8,
    paddingHorizontal: 16,
    justifyContent: 'center',
  },
  customAddText: {
    color: '#fff',
    fontWeight: '600',
    fontSize: 13,
  },
  projectItem: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    paddingVertical: 14,
    paddingHorizontal: 12,
    borderRadius: 8,
  },
  projectItemActive: {
    backgroundColor: '#4f7cff22',
  },
  projectItemText: {
    color: '#e4e6eb',
    fontSize: 15,
  },
  projectItemTextActive: {
    color: '#4f7cff',
    fontWeight: '600',
  },
  checkmark: {
    color: '#4f7cff',
    fontSize: 14,
    fontWeight: '700',
  },
});
