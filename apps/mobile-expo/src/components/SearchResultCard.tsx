import React from 'react';
import { View, Text, StyleSheet, TouchableOpacity } from 'react-native';
import type { SearchResult } from '../types';

interface SearchResultCardProps {
  result: SearchResult;
  onPress?: (r: SearchResult) => void;
}

export default function SearchResultCard({ result, onPress }: SearchResultCardProps) {
  return (
    <TouchableOpacity style={styles.card} onPress={() => onPress?.(result)} activeOpacity={0.7}>
      <View style={styles.header}>
        <Text style={styles.title} numberOfLines={1}>
          {result.title}
        </Text>
        <View style={styles.typeBadge}>
          <Text style={styles.typeText}>{result.type}</Text>
        </View>
      </View>
      <Text style={styles.snippet} numberOfLines={3}>
        {result.snippet}
      </Text>
      <Text style={styles.path}>{result.path}</Text>
    </TouchableOpacity>
  );
}

const styles = StyleSheet.create({
  card: {
    backgroundColor: '#1a1d27',
    borderRadius: 12,
    padding: 14,
    marginBottom: 10,
    borderWidth: 1,
    borderColor: '#252836',
  },
  header: {
    flexDirection: 'row',
    alignItems: 'center',
    marginBottom: 8,
  },
  title: {
    flex: 1,
    color: '#e4e6eb',
    fontSize: 15,
    fontWeight: '600',
  },
  typeBadge: {
    backgroundColor: '#2a2f3f',
    borderRadius: 6,
    paddingHorizontal: 8,
    paddingVertical: 3,
    marginLeft: 10,
  },
  typeText: {
    color: '#4f7cff',
    fontSize: 11,
    fontWeight: '600',
    textTransform: 'uppercase',
  },
  snippet: {
    color: '#9ca3af',
    fontSize: 13,
    lineHeight: 18,
    marginBottom: 6,
  },
  path: {
    color: '#6b7280',
    fontSize: 11,
  },
});
