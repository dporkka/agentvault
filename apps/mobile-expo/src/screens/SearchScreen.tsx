import React, { useState, useCallback } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  FlatList,
  ActivityIndicator,
  StyleSheet,
} from 'react-native';
import { searchVault } from '../api/agentvault';
import type { SearchResult } from '../types';
import SearchResultCard from '../components/SearchResultCard';
import ConnectionBadge from '../components/ConnectionBadge';

export default function SearchScreen() {
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [hasSearched, setHasSearched] = useState(false);

  const handleSearch = useCallback(async () => {
    const q = query.trim();
    if (!q) return;
    setLoading(true);
    setError('');
    setHasSearched(true);
    try {
      const data = await searchVault(q);
      setResults(data);
    } catch {
      setError('Search failed. Server may be offline.');
      setResults([]);
    }
    setLoading(false);
  }, [query]);

  return (
    <View style={styles.container}>
      <View style={styles.header}>
        <Text style={styles.title}>Search Vault</Text>
        <ConnectionBadge />
      </View>

      <View style={styles.searchRow}>
        <TextInput
          style={styles.input}
          placeholder="Search your vault..."
          placeholderTextColor="#6b7280"
          value={query}
          onChangeText={setQuery}
          onSubmitEditing={handleSearch}
          returnKeyType="search"
          autoCapitalize="none"
        />
        <TouchableOpacity style={styles.searchBtn} onPress={handleSearch}>
          <Text style={styles.searchBtnText}>Search</Text>
        </TouchableOpacity>
      </View>

      {loading && (
        <ActivityIndicator style={styles.loader} color="#4f7cff" size="large" />
      )}

      {error ? (
        <View style={styles.errorBox}>
          <Text style={styles.errorText}>{error}</Text>
        </View>
      ) : null}

      {!loading && hasSearched && results.length === 0 && !error ? (
        <View style={styles.empty}>
          <Text style={styles.emptyText}>No results found</Text>
        </View>
      ) : null}

      <FlatList
        data={results}
        keyExtractor={(item) => item.id}
        renderItem={({ item }) => <SearchResultCard result={item} />}
        contentContainerStyle={styles.list}
      />
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f1117',
    padding: 16,
  },
  header: {
    flexDirection: 'row',
    justifyContent: 'space-between',
    alignItems: 'center',
    marginBottom: 16,
    marginTop: 8,
  },
  title: {
    color: '#e4e6eb',
    fontSize: 22,
    fontWeight: '800',
  },
  searchRow: {
    flexDirection: 'row',
    gap: 10,
    marginBottom: 16,
  },
  input: {
    flex: 1,
    backgroundColor: '#1a1d27',
    borderRadius: 10,
    paddingHorizontal: 14,
    paddingVertical: 12,
    color: '#e4e6eb',
    fontSize: 15,
    borderWidth: 1,
    borderColor: '#252836',
  },
  searchBtn: {
    backgroundColor: '#4f7cff',
    borderRadius: 10,
    paddingHorizontal: 18,
    justifyContent: 'center',
  },
  searchBtnText: {
    color: '#fff',
    fontWeight: '700',
    fontSize: 14,
  },
  loader: {
    marginTop: 40,
  },
  errorBox: {
    backgroundColor: '#ef444422',
    borderRadius: 8,
    padding: 12,
    marginBottom: 16,
  },
  errorText: {
    color: '#ef4444',
    fontSize: 13,
    textAlign: 'center',
  },
  empty: {
    alignItems: 'center',
    paddingVertical: 40,
  },
  emptyText: {
    color: '#6b7280',
    fontSize: 15,
  },
  list: {
    paddingBottom: 20,
  },
});
