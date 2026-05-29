import React, { useState } from 'react';
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
import { addCapture } from '../storage/localInbox';
import { sendCapture } from '../api/agentvault';
import ProjectPicker from '../components/ProjectPicker';
import TagPicker from '../components/TagPicker';

export default function CaptureScreen() {
  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [project, setProject] = useState('');
  const [tags, setTags] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState('');
  const [isError, setIsError] = useState(false);

  const reset = () => {
    setTitle('');
    setBody('');
    setProject('');
    setTags([]);
    setMessage('');
  };

  const showMessage = (msg: string, error = false) => {
    setMessage(msg);
    setIsError(error);
    if (!error) setTimeout(() => setMessage(''), 2500);
  };

  const buildPayload = () => ({
    type: 'text' as const,
    title: title.trim() || (body.trim().slice(0, 50) || 'Untitled'),
    text: body.trim(),
    project: project || undefined,
    tags,
  });

  const handleSaveLocal = async () => {
    if (!title.trim() && !body.trim()) {
      showMessage('Enter a title or body', true);
      return;
    }
    setLoading(true);
    await addCapture(buildPayload());
    setLoading(false);
    showMessage('Saved to inbox');
    reset();
  };

  const handleSendNow = async () => {
    if (!title.trim() && !body.trim()) {
      showMessage('Enter a title or body', true);
      return;
    }
    setLoading(true);
    try {
      await sendCapture(buildPayload());
      showMessage('Sent to server!');
      reset();
    } catch (err) {
      showMessage('Server unreachable. Saved to inbox instead.', true);
      await addCapture(buildPayload());
    }
    setLoading(false);
  };

  return (
    <KeyboardAvoidingView
      style={styles.container}
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
            <Text style={[styles.toastText, isError && styles.toastTextError]}>
              {message}
            </Text>
          </View>
        ) : null}

        <Text style={styles.label}>Title</Text>
        <TextInput
          style={styles.input}
          placeholder="Capture title..."
          placeholderTextColor="#6b7280"
          value={title}
          onChangeText={setTitle}
          maxLength={200}
        />

        <Text style={styles.label}>Body</Text>
        <TextInput
          style={[styles.input, styles.textarea]}
          placeholder="Write your thoughts..."
          placeholderTextColor="#6b7280"
          value={body}
          onChangeText={setBody}
          multiline
          textAlignVertical="top"
          maxLength={5000}
        />
        <Text style={styles.charCount}>{body.length}/5000</Text>

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
            <Text style={styles.btnText}>
              {loading ? 'Sending...' : 'Send to Server'}
            </Text>
          </TouchableOpacity>
        </View>
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  container: {
    flex: 1,
    backgroundColor: '#0f1117',
  },
  scroll: {
    flex: 1,
  },
  content: {
    padding: 16,
    paddingBottom: 40,
  },
  header: {
    color: '#e4e6eb',
    fontSize: 22,
    fontWeight: '800',
    marginBottom: 16,
  },
  toast: {
    backgroundColor: '#22c55e22',
    borderRadius: 8,
    padding: 10,
    marginBottom: 14,
    alignItems: 'center',
  },
  toastError: {
    backgroundColor: '#ef444422',
  },
  toastText: {
    color: '#22c55e',
    fontSize: 13,
    fontWeight: '500',
  },
  toastTextError: {
    color: '#ef4444',
  },
  label: {
    color: '#9ca3af',
    fontSize: 12,
    fontWeight: '600',
    marginBottom: 6,
    marginTop: 8,
    textTransform: 'uppercase',
    letterSpacing: 0.5,
  },
  input: {
    backgroundColor: '#1a1d27',
    borderRadius: 8,
    paddingHorizontal: 12,
    paddingVertical: 12,
    color: '#e4e6eb',
    fontSize: 15,
    borderWidth: 1,
    borderColor: '#252836',
  },
  textarea: {
    minHeight: 120,
    lineHeight: 20,
  },
  charCount: {
    color: '#6b7280',
    fontSize: 11,
    textAlign: 'right',
    marginTop: 4,
  },
  actions: {
    flexDirection: 'row',
    gap: 10,
    marginTop: 20,
  },
  btn: {
    flex: 1,
    borderRadius: 10,
    paddingVertical: 14,
    alignItems: 'center',
  },
  btnPrimary: {
    backgroundColor: '#4f7cff',
  },
  btnSecondary: {
    backgroundColor: '#1a1d27',
    borderWidth: 1,
    borderColor: '#4f7cff',
  },
  btnDisabled: {
    opacity: 0.5,
  },
  btnText: {
    color: '#fff',
    fontSize: 15,
    fontWeight: '700',
  },
  btnTextSecondary: {
    color: '#4f7cff',
  },
});
