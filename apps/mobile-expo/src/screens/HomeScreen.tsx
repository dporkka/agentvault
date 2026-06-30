import React, { useState, useCallback, useEffect } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  ScrollView,
  RefreshControl,
  ActivityIndicator,
  StyleSheet,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { getDashboard } from '../api/agentvault';
import { addCapture } from '../storage/localInbox';
import ConnectionBadge from '../components/ConnectionBadge';
import { colors, spacing, radii, typography } from '../theme';
import type { RootTabScreenProps } from '../navigation/types';
import type { DashboardResponse, TaskResult, DecisionResult } from '@agentvault/contract';

function formatDueDate(iso?: string): string {
  if (!iso) return 'No due date';
  const date = new Date(iso);
  if (Number.isNaN(date.getTime())) return iso;
  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    weekday: 'short',
  });
}

function priorityColor(priority?: string): string {
  switch (priority?.toLowerCase()) {
    case 'high':
    case 'urgent':
      return colors.error;
    case 'medium':
      return colors.warning;
    case 'low':
      return colors.success;
    default:
      return colors.textMuted;
  }
}

export default function HomeScreen({ navigation }: RootTabScreenProps<'Home'>) {
  const [quickTitle, setQuickTitle] = useState('');
  const [quickBody, setQuickBody] = useState('');
  const [message, setMessage] = useState('');

  const [dashboard, setDashboard] = useState<DashboardResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');

  const showMessage = (msg: string) => {
    setMessage(msg);
    setTimeout(() => setMessage(''), 2500);
  };

  const fetchDashboard = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const data = await getDashboard();
      setDashboard(data);
    } catch {
      setError('Could not load dashboard');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchDashboard();
  }, [fetchDashboard]);

  const handleQuickCapture = async () => {
    const title = quickTitle.trim() || undefined;
    const body = quickBody.trim();
    if (!title && !body) return;

    const captureTitle = title || (body.length > 50 ? body.slice(0, 50) + '...' : body);
    await addCapture({ type: 'text', title: captureTitle, text: body, tags: ['quick'] });
    setQuickTitle('');
    setQuickBody('');
    showMessage('Saved to inbox');
  };

  const handleTaskPress = (item: TaskResult | DecisionResult) => {
    navigation.navigate('NoteDetail', { id: item.id, title: item.title });
  };

  const hasAnyItems =
    (dashboard?.overdueTasks.length ?? 0) > 0 ||
    (dashboard?.upcomingTasks.length ?? 0) > 0 ||
    (dashboard?.pendingDecisions.length ?? 0) > 0;

  return (
    <SafeAreaView style={styles.container} edges={['top', 'left', 'right']}>
      <ScrollView
        style={styles.scroll}
        contentContainerStyle={styles.scrollContent}
        refreshControl={
          <RefreshControl refreshing={loading} onRefresh={fetchDashboard} tintColor={colors.accent} />
        }
      >
        <View style={styles.header}>
          <View>
            <Text style={styles.title}>Today</Text>
            <Text style={styles.subtitle}>Your actionable dashboard</Text>
          </View>
          <ConnectionBadge />
        </View>

        <View style={styles.quickCapture}>
          <TextInput
            style={styles.titleInput}
            placeholder="Title"
            placeholderTextColor={colors.textMuted}
            value={quickTitle}
            onChangeText={setQuickTitle}
            maxLength={120}
          />
          <TextInput
            style={styles.bodyInput}
            placeholder="Quick capture... jot an idea"
            placeholderTextColor={colors.textMuted}
            value={quickBody}
            onChangeText={setQuickBody}
            multiline
            maxLength={500}
          />
          <View style={styles.quickActions}>
            <Text style={styles.charCount}>{quickBody.length}/500</Text>
            <TouchableOpacity
              style={[
                styles.captureBtn,
                !quickBody.trim() && !quickTitle.trim() && styles.captureBtnDisabled,
              ]}
              onPress={handleQuickCapture}
              disabled={!quickBody.trim() && !quickTitle.trim()}
            >
              <Text style={styles.captureBtnText}>Capture</Text>
            </TouchableOpacity>
          </View>
        </View>

        {message ? (
          <View style={styles.toast}>
            <Text style={styles.toastText}>{message}</Text>
          </View>
        ) : null}

        {loading && !dashboard ? (
          <ActivityIndicator style={styles.loader} color={colors.accent} size="large" />
        ) : null}

        {error ? (
          <View style={styles.errorBox}>
            <Text style={styles.errorText}>{error}</Text>
          </View>
        ) : null}

        {!loading && dashboard && !hasAnyItems ? (
          <View style={styles.empty}>
            <Text style={styles.emptyText}>All caught up</Text>
            <Text style={styles.emptySub}>No overdue tasks, upcoming tasks, or pending decisions.</Text>
          </View>
        ) : null}

        {dashboard && dashboard.overdueTasks.length > 0 ? (
          <View style={styles.section}>
            <Text style={[styles.sectionTitle, styles.overdueTitle]}>Overdue</Text>
            {dashboard.overdueTasks.map((item) => (
              <TouchableOpacity
                key={item.id}
                style={[styles.card, styles.overdueCard]}
                onPress={() => handleTaskPress(item)}
                activeOpacity={0.7}
              >
                <View style={styles.cardHeader}>
                  <Text style={styles.cardTitle} numberOfLines={1}>
                    {item.title}
                  </Text>
                  <View style={[styles.badge, { borderColor: colors.error }]}>
                    <Text style={[styles.badgeText, { color: colors.error }]}>{item.priority}</Text>
                  </View>
                </View>
                <Text style={styles.dueDate}>{formatDueDate(item.dueDate)}</Text>
              </TouchableOpacity>
            ))}
          </View>
        ) : null}

        {dashboard && dashboard.upcomingTasks.length > 0 ? (
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Upcoming</Text>
            {dashboard.upcomingTasks.map((item) => (
              <TouchableOpacity
                key={item.id}
                style={styles.card}
                onPress={() => handleTaskPress(item)}
                activeOpacity={0.7}
              >
                <View style={styles.cardHeader}>
                  <Text style={styles.cardTitle} numberOfLines={1}>
                    {item.title}
                  </Text>
                  <View style={[styles.badge, { borderColor: priorityColor(item.priority) }]}>
                    <Text style={[styles.badgeText, { color: priorityColor(item.priority) }]}>
                      {item.priority}
                    </Text>
                  </View>
                </View>
                <Text style={styles.dueDate}>{formatDueDate(item.dueDate)}</Text>
              </TouchableOpacity>
            ))}
          </View>
        ) : null}

        {dashboard && dashboard.pendingDecisions.length > 0 ? (
          <View style={styles.section}>
            <Text style={styles.sectionTitle}>Pending Decisions</Text>
            {dashboard.pendingDecisions.map((item) => (
              <TouchableOpacity
                key={item.id}
                style={styles.card}
                onPress={() => handleTaskPress(item)}
                activeOpacity={0.7}
              >
                <View style={styles.cardHeader}>
                  <Text style={styles.cardTitle} numberOfLines={1}>
                    {item.title}
                  </Text>
                  <View style={[styles.badge, { borderColor: colors.warning }]}>
                    <Text style={[styles.badgeText, { color: colors.warning }]}>{item.status}</Text>
                  </View>
                </View>
                <Text style={styles.path}>{item.path}</Text>
              </TouchableOpacity>
            ))}
          </View>
        ) : null}
      </ScrollView>
    </SafeAreaView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.bgPrimary,
  },
  scroll: {
    flex: 1,
  },
  scrollContent: {
    padding: spacing.lg,
    paddingBottom: spacing.xxl,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'flex-start',
    marginBottom: spacing.lg,
    marginTop: spacing.sm,
  },
  title: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xxxl,
    fontWeight: typography.weights.extrabold,
  },
  subtitle: {
    color: colors.textMuted,
    fontSize: typography.sizes.md,
    marginTop: 2,
  },
  quickCapture: {
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.xl,
    padding: 14,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
    marginBottom: spacing.md,
  },
  titleInput: {
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.semibold,
    paddingBottom: spacing.sm,
    borderBottomWidth: 1,
    borderBottomColor: colors.borderSubtle,
    marginBottom: spacing.sm,
  },
  bodyInput: {
    color: colors.textPrimary,
    fontSize: typography.sizes.base,
    minHeight: 60,
    textAlignVertical: 'top',
    lineHeight: 20,
  },
  quickActions: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginTop: 10,
  },
  charCount: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
  },
  captureBtn: {
    backgroundColor: colors.accent,
    borderRadius: radii.md,
    paddingHorizontal: 18,
    paddingVertical: spacing.sm,
  },
  captureBtnDisabled: {
    opacity: 0.4,
  },
  captureBtnText: {
    color: '#fff',
    fontWeight: typography.weights.semibold,
    fontSize: typography.sizes.base,
  },
  toast: {
    backgroundColor: `${colors.success}33`,
    borderRadius: radii.md,
    padding: 10,
    marginBottom: spacing.md,
    alignItems: 'center',
  },
  toastText: {
    color: colors.success,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.medium,
  },
  loader: {
    marginTop: 40,
  },
  errorBox: {
    backgroundColor: colors.errorMuted,
    borderRadius: radii.md,
    padding: spacing.md,
    marginBottom: spacing.md,
    alignItems: 'center',
  },
  errorText: {
    color: colors.error,
    fontSize: typography.sizes.md,
    textAlign: 'center',
  },
  empty: {
    alignItems: 'center',
    paddingVertical: 40,
  },
  emptyText: {
    color: colors.textMuted,
    fontSize: 16,
    fontWeight: typography.weights.semibold,
  },
  emptySub: {
    color: colors.textSecondary,
    fontSize: typography.sizes.md,
    marginTop: 6,
    textAlign: 'center',
  },
  section: {
    marginBottom: spacing.lg,
  },
  sectionTitle: {
    color: colors.textMuted,
    fontSize: typography.sizes.sm,
    fontWeight: typography.weights.bold,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
    marginBottom: spacing.sm,
  },
  overdueTitle: {
    color: colors.error,
  },
  card: {
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.xl,
    padding: 14,
    marginBottom: 10,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
  },
  overdueCard: {
    borderColor: `${colors.error}55`,
    backgroundColor: `${colors.error}11`,
  },
  cardHeader: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: spacing.sm,
  },
  cardTitle: {
    flex: 1,
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.semibold,
  },
  badge: {
    borderRadius: radii.sm,
    borderWidth: 1,
    paddingHorizontal: spacing.sm,
    paddingVertical: 2,
    marginLeft: spacing.sm,
  },
  badgeText: {
    fontSize: typography.sizes.xs,
    fontWeight: typography.weights.semibold,
    textTransform: 'uppercase',
  },
  dueDate: {
    color: colors.textSecondary,
    fontSize: typography.sizes.sm,
  },
  path: {
    color: colors.textMuted,
    fontSize: typography.sizes.sm,
  },
});
