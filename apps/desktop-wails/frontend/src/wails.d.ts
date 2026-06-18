// Type declarations for Wails runtime bindings.
//
// Data types (VaultStatus, Note, SearchResult, Answer, Source,
// IndexingStatus) live in `src/types/index.ts` so they can be shared
// with the HTTP clients via @agentvault/contract. The data type
// declarations used to live here as duplicate global interfaces;
// that drift is resolved in the contract-consolidation PR. The
// service bindings (method signatures on the Go bridge) stay here.

import type {
  VaultStatus,
  Note,
  SearchResult,
  Answer,
  IndexingStatus,
} from './types';

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

export {};
