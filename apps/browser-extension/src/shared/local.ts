// Local-only types used by the browser extension that are NOT part of the
// HTTP server contract. CapturePayload is the UI payload the popup builds
// before POSTing it to /capture; the server ignores any field it doesn't
// know about, so the extra `selectedText` and `capturedAt` are local.
// PageData describes what the content script extracts from the current
// page; it is sent across chrome.runtime messaging and never touches HTTP.

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

export interface PageData {
  title: string;
  url: string;
  selectedText: string;
  text?: string;
  author?: string;
  description?: string;
  publishedDate?: string;
}
