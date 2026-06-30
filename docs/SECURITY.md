# AgentVault Security Notes

AgentVault is designed as a **local-first, single-user** application. Your notes live as Markdown files you own, and the local API is intended to run on the same machine as the clients. This document describes the current security model and the risks of exposing it beyond that boundary.

## Threat model

The design assumes a **same-machine attacker model**: the vault and server are accessible only to the user running them and to other processes on the same machine. AgentVault does not protect against a compromised operating system or a malicious process running with the user's privileges. It is not a multi-user or network-hardened service.

In this model, the main goals are:

- Keep the vault data on the local machine by default.
- Prevent other users or remote attackers from reaching the API when bound to loopback.
- Use short-lived, per-startup tokens so that a leaked token is only useful while the server is running.

## Authentication

When `agentvault serve` starts it generates a fresh `X-AgentVault-Token` and prints it to the terminal. The token is a 32-character hex string generated with `crypto/rand` (16 bytes of randomness).

- `GET` endpoints are open locally.
- All `POST` / write endpoints require the token in either:
  - `X-AgentVault-Token: <token>`
  - `Authorization: Bearer <token>`

A missing or incorrect token returns `401 Unauthorized`.

Because the token changes every time the server restarts, clients store it locally and may need to be re-authenticated after a restart:

| Client | Storage |
| --- | --- |
| Web app (`apps/web-local`) | `localStorage` |
| Browser extension | `chrome.storage.local` |
| Desktop app | Wails runtime / in-memory bridge (no browser storage) |

These stores are only as secure as the operating system and browser profile they run in.

## Network boundary

`agentvault serve` binds to `127.0.0.1` by default. You can change the bind address with the `--host` flag, for example `--host 0.0.0.0`.

Binding to `0.0.0.0` makes the API reachable from other machines on the network. Do not do this on untrusted networks, because:

- The API is not hardened for remote exposure.
- `GET` endpoints are unauthenticated and can leak note titles, snippets, and search results.
- Write endpoints are protected only by the per-startup token, which is printed to stdout and may be visible in process listings.

Only change `--host` when you intentionally want another machine on the same trusted network to connect, and consider adding a separate reverse proxy or firewall rule instead.

## CORS

The local API reflects the request `Origin` for `file://`, `chrome-extension://`, `moz-extension://`, and any HTTP(S) origin whose host is exactly `localhost`, `127.0.0.1`, or `::1`. When no `Origin` header is present, the response allows `*`.

This permissive CORS policy is acceptable only because the server is expected to run on localhost. It must not be exposed to untrusted networks: an attacker who can reach the API from an arbitrary origin could read your indexed notes and, if they obtain the token, create or capture new notes.

## Browser extension permissions

The browser extension (`apps/browser-extension`) declares the following Manifest V3 permissions:

- `activeTab` — read the active tab's URL and page content when you invoke the extension.
- `storage` — persist the auth token, base URL, capture queue, and popup state.
- `contextMenus` — add the right-click "Capture to AgentVault" menu.
- `alarms` — retry queued captures in the background (in the production manifest).

Host permission is granted only for `http://127.0.0.1:47321/*` (or the configured local server URL).

The extension can read the active tab, page content, and any text you select. All captured data is sent only to your local AgentVault server; it is never sent to a third party.

## File system

The CLI runs with the privileges of the user who invokes it. Notes are stored as plain Markdown files with YAML frontmatter, owned by that user. There is no encryption at rest: anyone with read access to the vault directory can read the notes. Keep the vault directory protected by standard filesystem permissions.

The SQLite index (`<vault>/.agentvault/index.db`) is also owned by the user and contains the same searchable content.

## AI providers

Cloud AI providers (OpenAI, Anthropic, OpenRouter) require an API key. If you save one, it is stored in the vault configuration file, typically `<vault>/.agentvault/config.json`. The file is owned by the user and readable only with your OS permissions.

If you do not save a key in the config, the core can fall back to the `AGENTVAULT_API_KEY` environment variable.

Keep the vault directory private. Anyone with read access to it can read stored API keys and the full contents of your notes.

## Reporting issues

Security issues can be reported by opening a private issue or emailing the maintainers. Please include steps to reproduce and the affected version.
