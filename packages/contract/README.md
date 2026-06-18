# @agentvault/contract

Canonical TypeScript types and HTTP client for the AgentVault local API.
This package is the single source of truth for the request and response
shapes that the Go server emits; the four client apps consume it via
TypeScript path mappings (Vite + Metro `watchFolders`) instead of
duplicating types.

## Consumers

- `apps/web-local` — Vite + React, dev server on port 3000, talks to
  the local API at `http://127.0.0.1:47321`.
- `apps/browser-extension` — Vite + CRX, talks to the same local API
  from a popup, content script, or service worker.
- `apps/mobile-expo` — Expo + React Native + Metro, reads/writes the
  local API. Uses `AsyncStorage` as the token store.
- `apps/desktop-wails/frontend` — Vite + React, talks to the in-process
  Wails Go bridge. The Go bridge itself imports the matching structs
  from `core/internal/contract`, so the Wails frontend and the HTTP
  clients share one shape.

## Layout

```
packages/contract/
├── package.json
├── tsconfig.json
└── src/
    ├── types.ts       // the canonical request/response types
    ├── endpoints.ts   // typed route table
    ├── client.ts      // zero-dep HTTP client factory
    └── index.ts       // re-exports
```

The package has **no runtime dependencies** and **no build step**. The
clients consume the `.ts` files directly via TypeScript path mappings
and Metro `watchFolders`.

## Type-checking

```
npx tsc --noEmit
```

Or from the workspace root:

```
make contract-check
```

## Adding a new endpoint

1. Edit `core/internal/api/server.go` to register the route.
2. Add the new types to `packages/contract/src/types.ts`. Each type
   gets a header comment with the endpoint it comes from.
3. Add an entry to the `routes` constant in
   `packages/contract/src/endpoints.ts`.
4. Add a method to the `ApiClient` interface and `createClient` in
   `packages/contract/src/client.ts`.
5. The contract CI gate (`make contract-check`) will fail if the
   hand-written client types drift from the contract (it greps for
   snake_case keys and for hard-coded base URLs in client code).

## Adding a new field to an existing endpoint

If you add a field to a server response, just add it to the matching
TypeScript type in `types.ts` (with a comment). All four clients pick
it up on the next type-check. If you rename a field, also update
`docs/API_CONTRACT.md` and rerun the API shape tests in
`core/internal/api/server_test.go` to keep the contract locked down.
