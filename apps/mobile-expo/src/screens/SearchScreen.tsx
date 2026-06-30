import React, { useState, useCallback, useEffect, useRef } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  FlatList,
  ActivityIndicator,
  StyleSheet,
  Switch,
  KeyboardAvoidingView,
  Platform,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { useNavigation } from '@react-navigation/native';
import type { StackNavigationProp } from '@react-navigation/stack';
import Slider from '@react-native-community/slider';
import { searchVault, getRecentNotes } from '../api/agentvault';
import type { RootStackParamList } from '../navigation/types';
import type { SearchResult } from '../types';
import SearchResultCard from '../components/SearchResultCard';
import ConnectionBadge from '../components/ConnectionBadge';
import { colors, spacing, radii, typography } from '../theme';

type SearchNavigationProp = StackNavigationProp<RootStackParamList, 'MainTabs'>;

const SEARCH_DEBOUNCE_MS = 350;
const TOPK_MIN = 3;
const TOPK_MAX = 20;
const TOPK_STEP = 1;

export default function SearchScreen() {
  const navigation = useNavigation<SearchNavigationProp>();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [hasSearched, setHasSearched] = useState(false);
  const [vectorEnabled, setVectorEnabled] = useState(false);
  const [hybridWeight, setHybridWeight] = useState(0.5);
  const [topK, setTopK] = useState(10);
  const searchTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const performSearch = useCallback(
    async (q: string, opts?: { showLoading?: boolean }) => {
      const trimmed = q.trim();
      if (!trimmed) {
        setResults([]);
        setError('');
        return;
      }
      if (opts?.showLoading) setLoading(true);
      setError('');
      setHasSearched(true);
      try {
        const data = await searchVault({
          q: trimmed,
          vector: vectorEnabled || undefined,
          hybridWeight: vectorEnabled ? hybridWeight : undefined,
          limit: topK,
        });
        setResults(data);
      } catch {
        setError('Search failed. Server may be offline.');
        setResults([]);
      } finally {
        if (opts?.showLoading) setLoading(false);
      }
    },
    [vectorEnabled, hybridWeight, topK],
  );

  // Load recent notes on first mount and when topK changes while query is empty.
  useEffect(() => {
    if (query.trim()) return;
    let cancelled = false;
    (async () => {
      setLoading(true);
      setError('');
      setHasSearched(false);
      try {
        const data = await getRecentNotes(topK);
        if (!cancelled) setResults(data);
      } catch {
        if (!cancelled) setResults([]);
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [query, topK]);

  // Debounced live search while typing.
  useEffect(() => {
    if (searchTimeoutRef.current) clearTimeout(searchTimeoutRef.current);
    const trimmed = query.trim();
    if (!trimmed) return;
    searchTimeoutRef.current = setTimeout(() => {
      performSearch(query, { showLoading: true });
    }, SEARCH_DEBOUNCE_MS);
    return () => {
      if (searchTimeoutRef.current) clearTimeout(searchTimeoutRef.current);
    };
  }, [query, performSearch]);

  const handleSubmit = () => {
    if (searchTimeoutRef.current) clearTimeout(searchTimeoutRef.current);
    performSearch(query, { showLoading: true });
  };

  const adjustTopK = (delta: number) => {
    setTopK((prev) => Math.max(TOPK_MIN, Math.min(TOPK_MAX, prev + delta)));
  };

  const showEmptyState = !loading && results.length === 0 && !error;
  const showRecentHeader = !query.trim() && results.length > 0;

  return (
    <SafeAreaView style={styles.container} edges={['top', 'left', 'right']}>
      <KeyboardAvoidingView
        style={styles.keyboard}
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        keyboardVerticalOffset={80}
      >
        <View style={styles.header}>
          <Text style={styles.title}>Search Vault</Text>
          <ConnectionBadge />
        </View>

        <View style={styles.searchRow}>
          <TextInput
            style={styles.input}
            placeholder="Search your vault..."
            placeholderTextColor={colors.textMuted}
            value={query}
            onChangeText={setQuery}
            onSubmitEditing={handleSubmit}
            returnKeyType="search"
            autoCapitalize="none"
            clearButtonMode="while-editing"
          />
          <TouchableOpacity style={styles.searchBtn} onPress={handleSubmit}>
            <Text style={styles.searchBtnText}>Search</Text>
          </TouchableOpacity>
        </View>

        <View style={styles.optionRow}>
          <Text style={styles.optionLabel}>Vector search</Text>
          <Switch
            value={vectorEnabled}
            onValueChange={setVectorEnabled}
            trackColor={{ false: colors.borderSubtle, true: colors.accent }}
            thumbColor="#fff"
          />
        </View>

        {vectorEnabled && (
          <View style={styles.sliderSection}>
            <View style={styles.sliderHeader}>
              <Text style={styles.optionLabel}>Hybrid weight</Text>
              <Text style={styles.sliderValue}>{hybridWeight.toFixed(1)}</Text>
            </View>
            <Slider
              style={styles.slider}
              minimumValue={0}
              maximumValue={1}
              step={0.1}
              value={hybridWeight}
              onValueChange={setHybridWeight}
              minimumTrackTintColor={colors.accent}
              maximumTrackTintColor={colors.border}
              thumbTintColor={colors.accentHover}
            />
          </View>
        )}

        <View style={styles.optionRow}>
          <Text style={styles.optionLabel}>Results limit</Text>
          <View style={styles.stepper}>
            <TouchableOpacity
              style={styles.stepperBtn}
              onPress={() => adjustTopK(-TOPK_STEP)}
              disabled={topK <= TOPK_MIN}
            >
              <Text style={styles.stepperBtnText}>−</Text>
            </TouchableOpacity>
            <Text style={styles.stepperValue}>{topK}</Text>
            <TouchableOpacity
              style={styles.stepperBtn}
              onPress={() => adjustTopK(TOPK_STEP)}
              disabled={topK >= TOPK_MAX}
            >
              <Text style={styles.stepperBtnText}>+</Text>
            </TouchableOpacity>
          </View>
        </View>

        {showRecentHeader && <Text style={styles.sectionTitle}>Recent notes</Text>}

        {loading && <ActivityIndicator style={styles.loader} color={colors.accent} size="large" />}

        {error ? (
          <View style={styles.errorBox}>
            <Text style={styles.errorText}>{error}</Text>
          </View>
        ) : null}

        {showEmptyState ? (
          <View style={styles.empty}>
            <Text style={styles.emptyText}>
              {hasSearched ? 'No results found' : 'No recent notes'}
            </Text>
          </View>
        ) : null}

        <FlatList
          data={results}
          keyExtractor={(item) => item.id}
          renderItem={({ item }) => (
            <SearchResultCard
              result={item}
              onPress={() => navigation.navigate('NoteDetail', { id: item.id, title: item.title })}
            />
          )}
          contentContainerStyle={styles.list}
          keyboardShouldPersistTaps="handled"
        />
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
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: spacing.lg,
    marginTop: spacing.sm,
    paddingHorizontal: spacing.lg,
  },
  title: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xxl,
    fontWeight: typography.weights.extrabold,
  },
  searchRow: {
    flexDirection: 'row',
    gap: 10,
    marginBottom: spacing.lg,
    paddingHorizontal: spacing.lg,
  },
  input: {
    flex: 1,
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.lg,
    paddingHorizontal: 14,
    paddingVertical: spacing.md,
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
  },
  searchBtn: {
    backgroundColor: colors.accent,
    borderRadius: radii.lg,
    paddingHorizontal: 18,
    justifyContent: 'center',
  },
  searchBtnText: {
    color: '#fff',
    fontWeight: typography.weights.bold,
    fontSize: typography.sizes.base,
  },
  optionRow: {
    flexDirection: 'row',
    alignItems: 'center',
    justifyContent: 'space-between',
    marginBottom: spacing.md,
    paddingHorizontal: spacing.lg,
  },
  optionLabel: {
    color: colors.textSecondary,
    fontSize: typography.sizes.base,
  },
  sliderSection: {
    marginBottom: spacing.md,
    paddingHorizontal: spacing.lg,
  },
  sliderHeader: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: spacing.xs,
  },
  sliderValue: {
    color: colors.accent,
    fontSize: typography.sizes.base,
    fontWeight: typography.weights.semibold,
    fontVariant: ['tabular-nums'],
  },
  slider: {
    width: '100%',
    height: 32,
  },
  stepper: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: spacing.sm,
  },
  stepperBtn: {
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.sm,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
    width: 32,
    height: 32,
    alignItems: 'center',
    justifyContent: 'center',
  },
  stepperBtnText: {
    color: colors.accent,
    fontSize: typography.sizes.xl,
    fontWeight: typography.weights.bold,
  },
  stepperValue: {
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.semibold,
    minWidth: 28,
    textAlign: 'center',
    fontVariant: ['tabular-nums'],
  },
  sectionTitle: {
    color: colors.textMuted,
    fontSize: typography.sizes.sm,
    fontWeight: typography.weights.bold,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
    marginBottom: spacing.sm,
    paddingHorizontal: spacing.lg,
  },
  loader: {
    marginTop: 40,
  },
  errorBox: {
    backgroundColor: colors.errorMuted,
    borderRadius: radii.md,
    padding: spacing.md,
    marginBottom: spacing.lg,
    marginHorizontal: spacing.lg,
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
    fontSize: typography.sizes.lg,
  },
  list: {
    paddingBottom: spacing.xl,
    paddingHorizontal: spacing.lg,
  },
});
