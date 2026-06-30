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
import type { NoteDetail, NoteLink, NoteLinksResponse } from '@agentvault/contract';
import { getNote, getNoteLinks } from '../api/agentvault';
import type { RootStackScreenProps } from '../navigation/types';
import { colors, spacing, radii, typography } from '../theme';

export default function NoteDetailScreen({
  route,
  navigation,
}: RootStackScreenProps<'NoteDetail'>) {
  const { id, title } = route.params;
  const [note, setNote] = useState<NoteDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [links, setLinks] = useState<NoteLinksResponse | null>(null);
  const [linksLoading, setLinksLoading] = useState(false);
  const [linksError, setLinksError] = useState('');
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

  useEffect(() => {
    if (!note) return;
    let mounted = true;
    setLinksLoading(true);
    setLinksError('');
    getNoteLinks(note.id)
      .then((data) => {
        if (mounted) setLinks(data);
      })
      .catch((err) => {
        if (mounted) {
          setLinksError(err instanceof Error ? err.message : 'Could not load linked notes');
        }
      })
      .finally(() => {
        if (mounted) setLinksLoading(false);
      });
    return () => {
      mounted = false;
    };
  }, [note]);

  const handleLinkPress = (link: NoteLink) => {
    navigation.navigate('NoteDetail', { id: link.id, title: link.title });
  };

  const renderLinkItem = (link: NoteLink) => (
    <TouchableOpacity
      key={link.id}
      style={styles.linkItem}
      onPress={() => handleLinkPress(link)}
      activeOpacity={0.7}
    >
      <Text style={styles.linkTitle} numberOfLines={1}>
        {link.title}
      </Text>
      <Text style={styles.linkPath} numberOfLines={1}>
        {link.path}
      </Text>
    </TouchableOpacity>
  );

  const hasLinks = links && (links.backlinks.length > 0 || links.outgoing.length > 0);

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

      {loading && <ActivityIndicator style={styles.loader} color={colors.accent} size="large" />}

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

          <View style={styles.linksSection}>
            <Text style={styles.linksTitle}>Linked notes</Text>
            {linksLoading ? (
              <ActivityIndicator style={styles.linksLoader} color={colors.accent} size="small" />
            ) : linksError ? (
              <Text style={styles.linksError}>{linksError}</Text>
            ) : hasLinks ? (
              <>
                {links!.backlinks.length > 0 && (
                  <View style={styles.linkSubsection}>
                    <Text style={styles.linkSubsectionTitle}>Linked mentions</Text>
                    {links!.backlinks.map(renderLinkItem)}
                  </View>
                )}
                {links!.outgoing.length > 0 && (
                  <View style={styles.linkSubsection}>
                    <Text style={styles.linkSubsectionTitle}>Outgoing links</Text>
                    {links!.outgoing.map(renderLinkItem)}
                  </View>
                )}
              </>
            ) : (
              <Text style={styles.noLinks}>No linked notes</Text>
            )}
          </View>
        </ScrollView>
      )}
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.bgPrimary,
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    paddingHorizontal: spacing.lg,
    paddingBottom: spacing.md,
    borderBottomWidth: 1,
    borderBottomColor: colors.borderSubtle,
  },
  backBtn: {
    paddingRight: spacing.md,
  },
  backText: {
    color: colors.accent,
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.semibold,
  },
  headerTitle: {
    flex: 1,
    color: colors.textPrimary,
    fontSize: typography.sizes.xl,
    fontWeight: typography.weights.bold,
  },
  loader: {
    marginTop: 40,
  },
  errorBox: {
    backgroundColor: colors.errorMuted,
    borderRadius: radii.md,
    padding: spacing.md,
    margin: spacing.lg,
  },
  errorText: {
    color: colors.error,
    fontSize: typography.sizes.md,
    textAlign: 'center',
  },
  scroll: {
    flex: 1,
  },
  content: {
    padding: spacing.lg,
    paddingBottom: 40,
  },
  metaRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sm,
    marginBottom: 14,
  },
  typeBadge: {
    backgroundColor: colors.bgTertiary,
    borderRadius: radii.sm,
    paddingHorizontal: 10,
    paddingVertical: 4,
  },
  typeText: {
    color: colors.accent,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.bold,
    textTransform: 'uppercase',
  },
  projectBadge: {
    backgroundColor: `${colors.accent}22`,
    borderRadius: radii.sm,
    paddingHorizontal: 10,
    paddingVertical: 4,
    borderWidth: 1,
    borderColor: colors.accentMuted,
  },
  projectText: {
    color: colors.accent,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
  },
  statusBadge: {
    backgroundColor: colors.borderSubtle,
    borderRadius: radii.sm,
    paddingHorizontal: 10,
    paddingVertical: 4,
  },
  statusText: {
    color: colors.textSecondary,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
    textTransform: 'uppercase',
  },
  title: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xxl,
    fontWeight: typography.weights.extrabold,
    marginBottom: 6,
  },
  path: {
    color: colors.textMuted,
    fontSize: typography.sizes.sm,
    marginBottom: spacing.md,
  },
  tagsRow: {
    flexDirection: 'row',
    flexWrap: 'wrap',
    gap: spacing.sm,
    marginBottom: spacing.lg,
  },
  tag: {
    backgroundColor: colors.borderSubtle,
    borderRadius: radii.sm,
    paddingHorizontal: 10,
    paddingVertical: 4,
  },
  tagText: {
    color: colors.textSecondary,
    fontSize: typography.sizes.sm,
  },
  divider: {
    height: 1,
    backgroundColor: colors.borderSubtle,
    marginVertical: spacing.lg,
  },
  body: {
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    lineHeight: 22,
  },
  linksSection: {
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.xl,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
    padding: spacing.lg,
    marginTop: spacing.xl,
  },
  linksTitle: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xl,
    fontWeight: typography.weights.bold,
    marginBottom: spacing.md,
  },
  linksLoader: {
    marginVertical: spacing.md,
  },
  linksError: {
    color: colors.error,
    fontSize: typography.sizes.md,
    marginTop: spacing.sm,
  },
  linkSubsection: {
    marginTop: spacing.md,
  },
  linkSubsectionTitle: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.bold,
    textTransform: 'uppercase',
    marginBottom: spacing.sm,
  },
  linkItem: {
    paddingVertical: spacing.sm,
    borderBottomWidth: 1,
    borderBottomColor: colors.borderSubtle,
  },
  linkTitle: {
    color: colors.textPrimary,
    fontSize: typography.sizes.base,
    fontWeight: typography.weights.semibold,
    marginBottom: 2,
  },
  linkPath: {
    color: colors.textMuted,
    fontSize: typography.sizes.sm,
  },
  noLinks: {
    color: colors.textSecondary,
    fontSize: typography.sizes.md,
    fontStyle: 'italic',
  },
});
