# AgentVault Security Boundaries

AgentVault is a local-first application. This document describes the security model for the CLI, local HTTP API, browser extension, mobile clients, MCP server, and plugins.

## Localhost API

The CLI starts a local HTTP API with `agentvault serve`:

- **Default bind address:** `127.0.0.1:47321`
- **Public endpoints:** `GET /health`, `GET /auth/verify`, and CORS `OPTIONS` preflights.
- **Vault data endpoints:** require the auth token for both reads and writes.

Vault reads are sensitive: search results, note contents, projects, recent/stale notes, graph data, and Git status can all reveal private information. They therefore use the same authentication boundary as writes.

The server is intended to run on the same machine as the clients. Do not expose port `47321` to untrusted networks even when authentication is enabled.

## Auth token

- The token is generated randomly when `agentvault serve` starts.
- It is printed to standard output at startup.
- Web, extension, and mobile clients store the token locally and send it on protected requests in either:
  - The `X-AgentVault-Token` header, or
  - The `Authorization: Bearer <token>` header.
- `GET /auth/verify` is intentionally public so a client can ask whether a supplied token is valid. It does not return the server token.
- There is no token rotation or revocation while the server is running. Restarting `agentvault serve` generates a new token.
- Treat the token like a local password: do not share it, commit it, or log it.

## CORS

The local API uses a restrictive browser CORS policy:

- Allowed browser origins are local HTTP(S) origins whose host is exactly `localhost`, `127.0.0.1`, or `::1`, plus `file://`, `chrome-extension://`, and `moz-extension://` origins.
- Allowed browser origins are reflected exactly and may use credentials.
- Disallowed browser origins do not receive `Access-Control-Allow-Origin`.
- Requests without an `Origin` header are treated as non-browser clients. They may receive `Access-Control-Allow-Origin: *`, but the wildcard is never combined with `Access-Control-Allow-Credentials`.
- CORS is not an authentication mechanism. Protected endpoints still require the vault token.

The current extension-origin policy trusts the extension schemes broadly. A future pairing/trusted-extension-ID mechanism should narrow that boundary further.

## Browser extension

The browser extension is a Manifest V3 package:

- It communicates with `http://127.0.0.1:47321` (or the configured server URL).
- It requires the user-provided auth token for protected read and write operations.
- It does not read credentials, cookies, or page content outside of the active tab when the user triggers a capture.
- Only install the extension from a trusted release artifact or the Chrome Web Store once published.

## MCP server

The MCP HTTP transport can require the same AgentVault token. The stdio transport inherits the trust boundary of the process that launches it.

Built-in MCP tools expose behavior annotations (`readOnlyHint`, `destructiveHint`, `idempotentHint`, and `openWorldHint`) so clients can make safer tool-use decisions. These annotations are advisory metadata, not authorization controls.

## Plugins

Plugin manifests can declare `read`, `write`, and `annotate` permissions. Permission evaluation is **default-deny**: an omitted permission list grants no capability, and `write` includes the narrower `annotate` capability.

The current plugin package provides discovery and manifest/capability primitives. Do not treat manifest permissions as a complete sandbox: external MCP process isolation, filesystem/network restrictions, and capability enforcement at every future proxy invocation remain required before third-party plugins should be considered untrusted-code safe.

## Data storage

- Markdown files are the source of truth and live on your filesystem.
- The SQLite index (`<vault>/.agentvault/agentvault.db`) is a rebuildable cache.
- SQLite uses WAL mode and a bounded busy timeout for concurrent local access.
- Vault configuration (`<vault>/.agentvault/config.json`) may contain an AI provider API key; keep the `.agentvault` directory private.

## AI provider credentials and retrieved content

- Cloud AI providers require an API key.
- The key can be supplied via the `AGENTVAULT_API_KEY` environment variable or stored in the vault config.
- Do not commit API keys to version control.
- Content captured from webpages, imported notes, tool output, and other external sources must be treated as data rather than trusted instructions. Future memory/provenance work should preserve source trust and provenance explicitly so retrieved prompt-injection text cannot silently become policy.

## Desktop and mobile clients

- The desktop app runs the local API in-process via Wails; no external network port is opened by default.
- The mobile app is an Expo app that connects to the local API on the same network; use it only on trusted networks and configure the current server token.

## Reporting security issues

If you discover a security issue, please open a private issue or email the maintainer before disclosing it publicly.
