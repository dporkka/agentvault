export interface CapturePayload {
  type: 'text';
  title: string;
  text: string;
  project?: string;
  tags: string[];
}

export function buildPayload(
  title: string,
  body: string,
  project?: string,
  tags?: string[]
): CapturePayload {
  return {
    type: 'text',
    title: title.trim() || body.trim().slice(0, 50) || 'Untitled',
    text: body.trim(),
    project: project || undefined,
    tags: tags ?? [],
  };
}

export function validateCapture(title: string, body: string): string | null {
  if (!title.trim() && !body.trim()) {
    return 'Enter a title or body';
  }
  return null;
}
