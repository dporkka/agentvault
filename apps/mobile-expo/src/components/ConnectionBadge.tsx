import React from 'react';
import { View, Text, StyleSheet } from 'react-native';
import { useConnection } from '../hooks/useConnection';
import { colors, typography } from '../theme';

export default function ConnectionBadge() {
  const { status } = useConnection();

  const label = status === 'online' ? 'Connected' : status === 'offline' ? 'Offline' : '...';
  const color =
    status === 'online' ? colors.success : status === 'offline' ? colors.error : colors.textMuted;

  return (
    <View style={styles.container}>
      <View style={[styles.dot, { backgroundColor: color }]} />
      <Text style={[styles.text, { color }]}>{label}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flexDirection: 'row',
    alignItems: 'center',
    gap: 6,
  },
  dot: {
    width: 8,
    height: 8,
    borderRadius: 4,
  },
  text: {
    fontSize: typography.sizes.sm,
    fontWeight: typography.weights.medium,
  },
});
