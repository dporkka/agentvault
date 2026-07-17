const STORAGE_KEY = 'av-offline-queue';
const MAX_RETRIES = 3;

export interface QueuedRequest {
  id: string;
  method: string;
  path: string;
  body?: unknown;
  createdAt: number;
  retries: number;
}

function loadQueue(): QueuedRequest[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

function saveQueue(queue: QueuedRequest[]): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(queue));
}

export function enqueue(method: string, path: string, body?: unknown): void {
  const queue = loadQueue();
  queue.push({
    id: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
    method,
    path,
    body,
    createdAt: Date.now(),
    retries: 0,
  });
  saveQueue(queue);
}

export function getQueue(): QueuedRequest[] {
  return loadQueue();
}

export function removeFromQueue(id: string): void {
  const queue = loadQueue().filter(q => q.id !== id);
  saveQueue(queue);
}

export function clearQueue(): void {
  localStorage.removeItem(STORAGE_KEY);
}

export async function flushQueue(
  sender: (method: string, path: string, body?: unknown) => Promise<Response>,
): Promise<{ succeeded: number; failed: number }> {
  const queue = loadQueue();
  if (queue.length === 0) return { succeeded: 0, failed: 0 };

  let succeeded = 0;
  let failed = 0;
  const remaining: QueuedRequest[] = [];

  for (const req of queue) {
    try {
      const res = await sender(req.method, req.path, req.body);
      if (res.ok) {
        succeeded++;
        continue;
      }
    } catch {
      // Network error — keep in queue
    }

    req.retries++;
    if (req.retries < MAX_RETRIES) {
      remaining.push(req);
    } else {
      failed++;
    }
  }

  saveQueue(remaining);
  return { succeeded, failed };
}
