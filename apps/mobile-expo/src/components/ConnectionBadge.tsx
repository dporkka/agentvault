import React, { useEffect, useState } from 'react';
import { View, Text, StyleSheet } from 'react-native';
import NetInfo from '@react-native-community/netinfo';
import { checkHealth } from '../api/agentvault';

export default function ConnectionBadge() {
  const [status, setStatus] = useState<'online' | 'offline' | 'checking'>('checking');

  useEffect(() => {
    let mounted = true;
    const check = async () => {
      const netInfo = await NetInfo.fetch();
      if (!netInfo.isConnected) {
        if (mounted) setStatus('offline');
        return;
      }
      const healthy = await checkHealth();
      if (mounted) setStatus(healthy ? 'online' : 'offline');
    };
    check();
    const interval = setInterval(check, 15000);
    const unsub = NetInfo.addEventListener((state) => {
      if (!state.isConnected) setStatus('offline');
    });
    return () => {
      mounted = false;
      clearInterval(interval);
      unsub();
    };
  }, []);

  const label = status === 'online' ? 'Connected' : status === 'offline' ? 'Offline' : '...';
  const color = status === 'online' ? '#22c55e' : status === 'offline' ? '#ef4444' : '#6b7280';

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
    fontSize: 12,
    fontWeight: '500',
  },
});
