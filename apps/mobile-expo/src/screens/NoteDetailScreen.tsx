import React, { useEffect, useState } from 'react';
import {
  View,
  Text,
  ScrollView,
  TouchableOpacity,
  ActivityIndicator,
  StyleSheet,
} from 'react-native';
import { useSafeAreaInsets } from 'react-native-safe-area-context';
import type { NoteDetail } from '@agentvault/contract';
import { getNote } from '../api/agentvault';
import type { RootStackScreenProps } from '../navigation/types';

export default function NoteDetailScreen({
  route,
  navigation,
}: RootStackScreenProps<'NoteDetail'>) {
  const { id, title } = route.params;
  const [note, setNote] = useState<NoteDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const insets = useSafeAreaInsets();

  useEffect(() => {
    let mounted = true;
    getNote(id)
      .then((data) => {
        if (mounted) {
          setNote(data);
          navigation.setOptions({ title: data.title });
        }
      })
      .catch((err) => {
        if (mounted) {
          setError(err instanceof Error ? err.message : 'Could not load note');
        }
      })
      .finally(() => {
        if (mounted) setLoading(false);
      });
    return () => {
      mounted = false;
    };
  }, [id, navigation]);

  return (
    <View style={[styles.container, { paddingBottom: insets.bottom }]}>
      <View style={[styles.header, { paddingTop: insets.top + 8 }]}>
        <TouchableOpacity onPress={() => navigation.goBack()} style={styles.backBtn}>
          <Text style={styles.backText}>← Back</Text>
        </TouchableOpacity>
        <Text style={styles.headerTitle} numberOfLines={1}>
          {title}
        </Text>
      </View>

      {loading && (
        <ActivityIndicator style={styles.loader} color="#4f7cff" size="large" />
      )}

      {error ? (
        <View style={styles.errorBox}>
          <Text style={styles.errorText}>{error}</Text>
        </View>
      ) : null}

      {!loading && note && (
        <ScrollView style={styles.scroll} contentContainerStyle={styles.content}>
          <View style={styles.metaRow}>
            <View style={styles.typeBadge}>
              <Text style={styles.typeText}>{note.type}</Text>
            </View>
            {note.project ? (
              <View style={styles.projectBadge}>
                <Text style={styles.projectText}>{note.project}</Text>
              </View>
            ) : null}
            <View style={styles.statusBadge}>
              <Text style={styles.statusText}>{note.status}</Text>
            </View>
          </View>

          <Text style={styles.title}>{note.title}</Text>
          <Text style={styles.path}>{note.path}</Text>

          {note.tags.length > 0 && (
            <View style={styles.tagsRow}>
              {note.tags.map((tag) => (
                <View key={tag} style={styles.tag}>
                  <Text style={styles.tagText}>{tag}</Text>
                </View>
              ))}
            </View>
          )}

          <View style={styles.divider} />

          <Text style={styles.body}>{note.content}</Text>
        </ScrollView>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f1117',
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: 16,
    paddingBottom: 12,
    borderBottomWidth: 1,
    borderBottomColor: '#252836',
  },
  backBtn: {
    paddingRight: 12,
  },
  backText: {
    color: '#4f7cff',
    fontSize: 15,
    fontWeight: '600',
  },
  headerTitle: {
    flex: 1,
    color: '#e4e6eb',
    fontSize: 17,
    fontWeight: '700',
  },
  loader: {
    marginTop: 40,
  },
  errorBox: {
    backgroundColor: '#ef444422',
    borderRadius: 8,
    padding: 12,
    margin: 16,
  },
  errorText: {
    color: '#ef4444',
    fontSize: 13,
    textAlign: 'center',
  },
  scroll: {
    flex: 1,
  },
  content: {
    padding: 16,
    paddingBottom: 40,
  },
  metaRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginBottom: 14,
  },
  typeBadge: {
    backgroundColor: '#2a2f3f',
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 4,
  },
  typeText: {
    color: '#4f7cff',
    fontSize: 11,
    fontWeight: '700',
    textTransform: 'uppercase',
  },
  projectBadge: {
    backgroundColor: '#4f7cff22',
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderWidth: 1,
    borderColor: '#4f7cff33',
  },
  projectText: {
    color: '#4f7cff',
    fontSize: 11,
    fontWeight: '600',
  },
  statusBadge: {
    backgroundColor: '#252836',
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 4,
  },
  statusText: {
    color: '#9ca3af',
    fontSize: 11,
    fontWeight: '600',
    textTransform: 'uppercase',
  },
  title: {
    color: '#e4e6eb',
    fontSize: 22,
    fontWeight: '800',
    marginBottom: 6,
  },
  path: {
    color: '#6b7280',
    fontSize: 12,
    marginBottom: 12,
  },
  tagsRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: 8,
    marginBottom: 16,
  },
  tag: {
    backgroundColor: '#252836',
    borderRadius: 6,
    paddingHorizontal: 10,
    paddingVertical: 4,
  },
  tagText: {
    color: '#9ca3af',
    fontSize: 12,
  },
  divider: {
    height: 1,
    backgroundColor: '#252836',
    marginVertical: 16,
  },
  body: {
    color: '#e4e6eb',
    fontSize: 15,
    lineHeight: 22,
  },
});
