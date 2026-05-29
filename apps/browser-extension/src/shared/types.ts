export interface CapturePayload {
  type: 'webpage' | 'selection';
  title: string;
  url: string;
  text?: string;
  selectedText?: string;
  project?: string;
  tags?: string[];
  capturedAt: string;
}

export interface SearchResult {
  id: string;
  title: string;
  path: string;
  type: string;
  snippet: string;
}

export interface PageData {
  title: string;
  url: string;
  selectedText: string;
  author?: string;
  description?: string;
  publishedDate?: string;
}
