import React, { useState, useEffect } from 'react';
import {
  View,
  Text,
  TextInput,
  TouchableOpacity,
  ScrollView,
  KeyboardAvoidingView,
  Platform,
  StyleSheet,
} from 'react-native';
import { SafeAreaView } from 'react-native-safe-area-context';
import { addCapture } from '../storage/localInbox';
import { sendCapture } from '../api/agentvault';
import { useSettings } from '../context/SettingsContext';
import ProjectPicker from '../components/ProjectPicker';
import TagPicker from '../components/TagPicker';
import { colors, spacing, radii, typography } from '../theme';

type CaptureType = 'text' | 'webpage';

const CAPTURE_TYPES: { key: CaptureType; label: string }[] = [
  { key: 'text', label: 'Text' },
  { key: 'webpage', label: 'Webpage' },
];

export default function CaptureScreen() {
  const { settings } = useSettings();
  const [captureType, setCaptureType] = useState<CaptureType>('text');
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [url, setUrl] = useState('');
  const [project, setProject] = useState(settings.defaultProject || '');
  const [tags, setTags] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [isError, setIsError] = useState(false);

  useEffect(() => {
    if (settings.defaultProject && !project) {
      setProject(settings.defaultProject);
    }
  }, [settings.defaultProject]);

  const reset = () => {
    setCaptureType('text');
    setTitle('');
    setBody('');
    setUrl('');
    setProject(settings.defaultProject || '');
    setTags([]);
    setMessage('');
  };

  const showMessage = (msg: string, error = false) => {
    setMessage(msg);
    setIsError(error);
    if (!error) setTimeout(() => setMessage(''), 2500);
  };

  const buildPayload = () => {
    const trimmedTitle = title.trim();
    const trimmedBody = body.trim();
    const trimmedUrl = url.trim();
    if (captureType === 'webpage') {
      return {
        type: 'webpage' as const,
        title: trimmedTitle || trimmedUrl.slice(0, 50) || 'Untitled',
        url: trimmedUrl,
        project: project || undefined,
        tags,
      };
    }
    return {
      type: 'text' as const,
      title: trimmedTitle || trimmedBody.slice(0, 50) || 'Untitled',
      text: trimmedBody,
      project: project || undefined,
      tags,
    };
  };

  const isPayloadEmpty = () => {
    if (captureType === 'webpage') {
      return !title.trim() && !url.trim();
    }
    return !title.trim() && !body.trim();
  };

  const handleSaveLocal = async () => {
    if (isPayloadEmpty()) {
      showMessage(
        captureType === 'webpage' ? 'Enter a title or URL' : 'Enter a title or body',
        true,
      );
      return;
    }
    setLoading(true);
    await addCapture(buildPayload());
    setLoading(false);
    showMessage('Saved to inbox');
    reset();
  };

  const handleSendNow = async () => {
    if (isPayloadEmpty()) {
      showMessage(
        captureType === 'webpage' ? 'Enter a title or URL' : 'Enter a title or body',
        true,
      );
      return;
    }
    setLoading(true);
    try {
      await sendCapture(buildPayload());
      showMessage('Sent to server!');
      reset();
    } catch {
      showMessage('Server unreachable. Saved to inbox instead.', true);
      await addCapture(buildPayload());
    }
    setLoading(false);
  };

  return (
    <SafeAreaView style={styles.container} edges={['top', 'left', 'right']}>
      <KeyboardAvoidingView
        style={styles.keyboard}
        behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        keyboardVerticalOffset={80}
      >
        <ScrollView
          style={styles.scroll}
          contentContainerStyle={styles.content}
          keyboardShouldPersistTaps="handled"
        >
          <Text style={styles.header}>New Capture</Text>

          {message ? (
            <View style={[styles.toast, isError && styles.toastError]}>
              <Text style={[styles.toastText, isError && styles.toastTextError]}>{message}</Text>
            </View>
          ) : null}

          <View style={styles.typeToggle}>
            {CAPTURE_TYPES.map(({ key, label }) => (
              <TouchableOpacity
                key={key}
                style={[styles.typeBtn, captureType === key && styles.typeBtnActive]}
                onPress={() => setCaptureType(key)}
                accessibilityState={{ selected: captureType === key }}
              >
                <Text style={[styles.typeBtnText, captureType === key && styles.typeBtnTextActive]}>
                  {label}
                </Text>
              </TouchableOpacity>
            ))}
          </View>

          <Text style={styles.label}>Title</Text>
          <TextInput
            style={styles.input}
            placeholder="Capture title..."
            placeholderTextColor={colors.textMuted}
            value={title}
            onChangeText={setTitle}
            maxLength={200}
          />

          {captureType === 'webpage' ? (
            <>
              <Text style={styles.label}>URL</Text>
              <TextInput
                style={styles.input}
                placeholder="https://..."
                placeholderTextColor={colors.textMuted}
                value={url}
                onChangeText={setUrl}
                autoCapitalize="none"
                autoCorrect={false}
                keyboardType="url"
                maxLength={2048}
              />
              <Text style={styles.charCount}>{url.length}/2048</Text>
            </>
          ) : (
            <>
              <Text style={styles.label}>Body</Text>
              <TextInput
                style={[styles.input, styles.textarea]}
                placeholder="Write your thoughts..."
                placeholderTextColor={colors.textMuted}
                value={body}
                onChangeText={setBody}
                multiline
                textAlignVertical="top"
                maxLength={5000}
              />
              <Text style={styles.charCount}>{body.length}/5000</Text>
            </>
          )}

          <ProjectPicker selected={project} onChange={setProject} />

          <TagPicker selected={tags} onChange={setTags} />

          <View style={styles.actions}>
            <TouchableOpacity
              style={[styles.btn, styles.btnSecondary, loading && styles.btnDisabled]}
              onPress={handleSaveLocal}
              disabled={loading}
            >
              <Text style={[styles.btnText, styles.btnTextSecondary]}>
                {loading ? 'Saving...' : 'Save to Inbox'}
              </Text>
            </TouchableOpacity>

            <TouchableOpacity
              style={[styles.btn, styles.btnPrimary, loading && styles.btnDisabled]}
              onPress={handleSendNow}
              disabled={loading}
            >
              <Text style={styles.btnText}>{loading ? 'Sending...' : 'Send to Server'}</Text>
            </TouchableOpacity>
          </View>
        </ScrollView>
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
  scroll: {
    flex: 1,
  },
  content: {
    padding: spacing.lg,
    paddingBottom: 40,
  },
  header: {
    color: colors.textPrimary,
    fontSize: typography.sizes.xxl,
    fontWeight: typography.weights.extrabold,
    marginBottom: spacing.lg,
  },
  typeToggle: {
    flexDirection: 'row',
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.lg,
    padding: 4,
    marginBottom: spacing.lg,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
  },
  typeBtn: {
    flex: 1,
    alignItems: 'center',
    paddingVertical: 8,
    borderRadius: radii.md,
  },
  typeBtnActive: {
    backgroundColor: colors.accent,
  },
  typeBtnText: {
    color: colors.textSecondary,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.semibold,
  },
  typeBtnTextActive: {
    color: '#fff',
  },
  toast: {
    backgroundColor: colors.successMuted,
    borderRadius: radii.md,
    padding: 10,
    marginBottom: 14,
    alignItems: 'center',
  },
  toastError: {
    backgroundColor: colors.errorMuted,
  },
  toastText: {
    color: colors.success,
    fontSize: typography.sizes.md,
    fontWeight: typography.weights.medium,
  },
  toastTextError: {
    color: colors.error,
  },
  label: {
    color: colors.textSecondary,
    fontSize: typography.sizes.sm,
    fontWeight: typography.weights.semibold,
    marginBottom: 6,
    marginTop: spacing.sm,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  input: {
    backgroundColor: colors.bgSecondary,
    borderRadius: radii.md,
    paddingHorizontal: spacing.md,
    paddingVertical: spacing.md,
    color: colors.textPrimary,
    fontSize: typography.sizes.lg,
    borderWidth: 1,
    borderColor: colors.borderSubtle,
  },
  textarea: {
    minHeight: 120,
    lineHeight: 20,
  },
  charCount: {
    color: colors.textMuted,
    fontSize: typography.sizes.xs,
    textAlign: 'right',
    marginTop: spacing.xs,
  },
  actions: {
    flexDirection: 'row',
    gap: 10,
    marginTop: spacing.xl,
  },
  btn: {
    flex: 1,
    borderRadius: radii.lg,
    paddingVertical: 14,
    alignItems: 'center',
  },
  btnPrimary: {
    backgroundColor: colors.accent,
  },
  btnSecondary: {
    backgroundColor: colors.bgSecondary,
    borderWidth: 1,
    borderColor: colors.accent,
  },
  btnDisabled: {
    opacity: 0.5,
  },
  btnText: {
    color: '#fff',
    fontSize: typography.sizes.lg,
    fontWeight: typography.weights.bold,
  },
  btnTextSecondary: {
    color: colors.accent,
  },
});
