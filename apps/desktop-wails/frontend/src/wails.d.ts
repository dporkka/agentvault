// Type declarations for Wails runtime bindings

declare global {
  interface Window {
    go: {
      main: {
        App: any;
        VaultService: VaultServiceBindings;
        NoteService: NoteServiceBindings;
        IndexService: IndexServiceBindings;
        AIService: AIServiceBindings;
      };
    };
    runtime: any;
  }
}

interface VaultServiceBindings {
  GetVaultPath(): Promise<string>;
  IsVault(path: string): Promise<boolean>;
  InitVault(path: string): Promise<void>;
  OpenVault(path: string): Promise<void>;
  GetStatus(): Promise<VaultStatus>;
  SelectFolder(): Promise<string>;
}

interface NoteServiceBindings {
  Search(query: string, noteType: string, project: string): Promise<SearchResult[]>;
  GetNote(id: string): Promise<Note>;
  GetNoteContent(path: string): Promise<string>;
  SaveNote(path: string, content: string): Promise<void>;
  CreateNote(noteType: string, title: string, project: string): Promise<string>;
  GetRecent(limit: number): Promise<SearchResult[]>;
  GetProjects(): Promise<string[]>;
  GetNotesByProject(project: string): Promise<SearchResult[]>;
}

interface IndexServiceBindings {
  Index(force: boolean): Promise<void>;
  GetStatus(): Promise<IndexingStatus>;
}

interface AIServiceBindings {
  Ask(question: string): Promise<Answer>;
  IsAIEnabled(): Promise<boolean>;
}

// Data types
interface VaultStatus {
  path: string;
  isOpen: boolean;
  noteCount: number;
  version: string;
}

interface Note {
  id: string;
  title: string;
  path: string;
  type: string;
  project: string;
  status: string;
  tags: string[];
  body: string;
  updatedAt: string;
}

interface SearchResult {
  id: string;
  title: string;
  path: string;
  type: string;
  project: string;
  tags: string[];
  snippet: string;
  updatedAt: string;
}

interface IndexingStatus {
  isIndexing: boolean;
  noteCount: number;
}

interface Answer {
  answer: string;
  sources: Source[];
  confidence: string;
  caveats: string[];
  missingInfo: string;
  suggestedActions: string[];
}

interface Source {
  path: string;
  title: string;
  excerpt: string;
}

export {};
