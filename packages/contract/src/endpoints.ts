// Typed route table for the AgentVault HTTP API. Each entry binds a
// method+path to its request and response payload types, so call sites
// can do `Endpoint<'POST', '/notes'>` to get a `{ request, response }`
// pair with the right TypeScript types.

import type {
  AskRequest,
  AskResponse,
  AuthVerifyResponse,
  CaptureRequest,
  CaptureResponse,
  CreateNoteRequest,
  CreateNoteResponse,
  GitStatus,
  HealthResponse,
  IndexOptions,
  IndexResult,
  NoteDetail,
  Projects,
  SearchResult,
  SearchParams,
  VaultStatus,
} from './types';

type Method = 'GET' | 'POST';

export interface EndpointDef<Req, Res> {
  method: Method;
  path: string;
  auth: boolean;
  request: Req;
  response: Res;
}

export type NoRequest = never;

// routes is the authoritative list. Anything not here is not a server
// route. Keep this list in sync with `core/internal/api/server.go`'s
// RegisterRoutes and with docs/API_CONTRACT.md.
export const routes: {
  readonly health: EndpointDef<NoRequest, HealthResponse>;
  readonly authVerify: EndpointDef<NoRequest, AuthVerifyResponse>;
  readonly vaultStatus: EndpointDef<NoRequest, VaultStatus>;
  readonly vaultIndex: EndpointDef<IndexOptions | undefined, IndexResult>;
  readonly search: EndpointDef<SearchParams, SearchResult[]>;
  readonly noteById: EndpointDef<{ id: string }, NoteDetail>;
  readonly createNote: EndpointDef<CreateNoteRequest, CreateNoteResponse>;
  readonly capture: EndpointDef<CaptureRequest, CaptureResponse>;
  readonly ask: EndpointDef<AskRequest, AskResponse>;
  readonly projects: EndpointDef<NoRequest, Projects>;
  readonly recent: EndpointDef<NoRequest, SearchResult[]>;
  readonly stale: EndpointDef<NoRequest, SearchResult[]>;
  readonly gitStatus: EndpointDef<NoRequest, GitStatus>;
} = {
  health: {
    method: 'GET',
    path: '/health',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as HealthResponse,
  },
  authVerify: {
    method: 'GET',
    path: '/auth/verify',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as AuthVerifyResponse,
  },
  vaultStatus: {
    method: 'GET',
    path: '/vault/status',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as VaultStatus,
  },
  vaultIndex: {
    method: 'POST',
    path: '/vault/index',
    auth: true,
    request: undefined as IndexOptions | undefined,
    response: undefined as unknown as IndexResult,
  },
  search: {
    method: 'GET',
    path: '/search',
    auth: false,
    request: undefined as unknown as SearchParams,
    response: undefined as unknown as SearchResult[],
  },
  noteById: {
    method: 'GET',
    path: '/notes/{id}',
    auth: false,
    request: undefined as unknown as { id: string },
    response: undefined as unknown as NoteDetail,
  },
  createNote: {
    method: 'POST',
    path: '/notes',
    auth: true,
    request: undefined as unknown as CreateNoteRequest,
    response: undefined as unknown as CreateNoteResponse,
  },
  capture: {
    method: 'POST',
    path: '/capture',
    auth: true,
    request: undefined as unknown as CaptureRequest,
    response: undefined as unknown as CaptureResponse,
  },
  ask: {
    method: 'POST',
    path: '/ask',
    auth: true,
    request: undefined as unknown as AskRequest,
    response: undefined as unknown as AskResponse,
  },
  projects: {
    method: 'GET',
    path: '/projects',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as Projects,
  },
  recent: {
    method: 'GET',
    path: '/recent',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as SearchResult[],
  },
  stale: {
    method: 'GET',
    path: '/stale',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as SearchResult[],
  },
  gitStatus: {
    method: 'GET',
    path: '/git/status',
    auth: false,
    request: undefined as never,
    response: undefined as unknown as GitStatus,
  },
};

// Endpoint is preserved for internal use; callers typically index
// into `routes` directly by key.
export type Endpoint<
  M extends Method,
  P extends string,
> = (typeof routes)[KeyOfRoutesWith<M, P>];

type KeyOfRoutesWith<M extends Method, P extends string> = {
  [K in keyof typeof routes]: (typeof routes)[K] extends { method: M; path: P }
    ? K
    : never;
}[keyof typeof routes];
