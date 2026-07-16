import React, { useState, useCallback } from 'react';
import { View, Text, TouchableOpacity, Alert, StyleSheet } from 'react-native';
import { CameraView, useCameraPermissions } from 'expo-camera';
import { useSettings } from '../context/SettingsContext';
import { colors, spacing, radii, typography } from '../theme';
import type { RootStackScreenProps } from '../navigation/types';

/**
 * QRScannerScreen opens the device camera and scans for QR codes.
 * When a valid AgentVault URL is scanned (http://HOST:PORT?token=TOKEN),
 * it saves the server URL and token to settings.
 */
export default function QRScannerScreen({ navigation }: RootStackScreenProps<'QRScanner'>) {
  const [permission, requestPermission] = useCameraPermissions();
  const { settings, saveSettings } = useSettings();
  const [scanned, setScanned] = useState(false);

  const handleBarCodeScanned = useCallback(
    async ({ data }: { data: string }) => {
      if (scanned) return;
      setScanned(true);

      try {
        // Parse the QR URL: http://HOST:PORT?token=TOKEN
        const url = new URL(data);
        const token = url.searchParams.get('token');
        if (!token) {
          Alert.alert('Invalid QR', 'This QR code does not contain an AgentVault connection URL.');
          setScanned(false);
          return;
        }

        const baseUrl = `${url.protocol}//${url.host}`;
        await saveSettings({ ...settings, serverUrl: baseUrl, token });

        Alert.alert('Connected!', `Server: ${baseUrl}\nToken saved.`, [
          { text: 'OK', onPress: () => navigation.goBack() },
        ]);
      } catch {
        Alert.alert('Invalid QR', 'Could not parse the QR code as a valid URL.');
        setScanned(false);
      }
    },
    [scanned, settings, saveSettings, navigation],
  );

  if (!permission) {
    // Permissions are still loading
    return (
      <View style={styles.container}>
        <Text style={styles.loadingText}>Requesting camera permission...</Text>
      </View>
    );
  }

  if (!permission.granted) {
    return (
      <View style={styles.container}>
        <Text style={styles.title}>Camera Access Required</Text>
        <Text style={styles.subtitle}>
          Allow camera access to scan QR codes from the AgentVault CLI.
        </Text>
        <TouchableOpacity style={styles.permissionBtn} onPress={requestPermission}>
          <Text style={styles.permissionBtnText}>Grant Permission</Text>
        </TouchableOpacity>
        <TouchableOpacity
          style={styles.manualBtn}
          onPress={() => navigation.goBack()}
        >
          <Text style={styles.manualBtnText}>Enter Manually</Text>
        </TouchableOpacity>
      </View>
    );
  }

  return (
    <View style={styles.container}>
      <CameraView
        style={styles.camera}
        facing="back"
        barcodeScannerSettings={{ barcodeTypes: ['qr'] }}
        onBarcodeScanned={scanned ? undefined : handleBarCodeScanned}
      />
      <View style={styles.overlay}>
        <View style={styles.scanRegion}>
          <View style={[styles.corner, styles.topLeft]} />
          <View style={[styles.corner, styles.topRight]} />
          <View style={[styles.corner, styles.bottomLeft]} />
          <View style={[styles.corner, styles.bottomRight]} />
        </View>
        <Text style={styles.hint}>Point camera at the QR code from{'\n'}the AgentVault terminal</Text>
        <TouchableOpacity
          style={styles.cancelBtn}
          onPress={() => navigation.goBack()}
        >
          <Text style={styles.cancelBtnText}>Cancel</Text>
        </TouchableOpacity>
      </View>
    </View>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: colors.bgPrimary,
    justifyContent: 'center',
    alignItems: 'center',
  },
  camera: {
    ...StyleSheet.absoluteFill,
  },
  overlay: {
    ...StyleSheet.absoluteFill,
    justifyContent: 'center',
    alignItems: 'center',
  },
  scanRegion: {
    width: 240,
    height: 240,
    justifyContent: 'space-between',
    alignItems: 'center',
  },
  corner: {
    position: 'absolute',
    width: 30,
    height: 30,
    borderColor: colors.accent,
  },
  topLeft: {
    top: 0,
    left: 0,
    borderTopWidth: 3,
    borderLeftWidth: 3,
  },
  topRight: {
    top: 0,
    right: 0,
    borderTopWidth: 3,
    borderRightWidth: 3,
  },
  bottomLeft: {
    bottom: 0,
    left: 0,
    borderBottomWidth: 3,
    borderLeftWidth: 3,
  },
  bottomRight: {
    bottom: 0,
    right: 0,
    borderBottomWidth: 3,
    borderRightWidth: 3,
  },
  hint: {
    color: colors.textInverse,
    fontSize: 14,
    textAlign: 'center',
    marginTop: 24,
    backgroundColor: 'rgba(0,0,0,0.5)',
    paddingHorizontal: 16,
    paddingVertical: 8,
    borderRadius: radii.sm,
    overflow: 'hidden',
  },
  cancelBtn: {
    marginTop: 32,
    backgroundColor: 'rgba(0,0,0,0.5)',
    paddingHorizontal: 24,
    paddingVertical: 10,
    borderRadius: radii.md,
  },
  cancelBtnText: {
    color: colors.textInverse,
    fontSize: 14,
    fontWeight: '600',
  },
  title: {
    fontSize: 18,
    fontWeight: '600',
    color: colors.textPrimary,
    marginBottom: 8,
  },
  subtitle: {
    fontSize: 14,
    color: colors.textSecondary,
    textAlign: 'center',
    marginBottom: 24,
    paddingHorizontal: 32,
  },
  loadingText: {
    fontSize: 14,
    color: colors.textSecondary,
  },
  permissionBtn: {
    backgroundColor: colors.accent,
    paddingHorizontal: 24,
    paddingVertical: 12,
    borderRadius: radii.md,
    marginBottom: 12,
  },
  permissionBtnText: {
    color: colors.textInverse,
    fontSize: 14,
    fontWeight: '600',
  },
  manualBtn: {
    paddingHorizontal: 24,
    paddingVertical: 12,
  },
  manualBtnText: {
    color: colors.accent,
    fontSize: 14,
  },
});
