# AgentVault Security Boundaries

AgentVault is a local-first application. This document describes the security model for the CLI, local HTTP API, browser extension, and mobile clients.

## Localhost API

The CLI starts a local HTTP API with `agentvault serve`:

- **Default bind address:** `127.0.0.1:47321`
- **Read endpoints (`GET`):** open to any client that can reach localhost.
- **Write endpoints (`POST` / `PUT` / `DELETE`):** require the auth token.

The server is intended to run on the same machine as the clients. Do not expose port `47321` to untrusted networks.

## Auth token

- The token is generated randomly when `agentvault serve` starts.
- It is printed to standard output at startup.
- Web, extension, and mobile clients store the token locally and send it in either:
  - The `X-AgentVault-Token` header, or
  - The `Authorization: Bearer <token>` header.
- There is no token rotation or revocation while the server is running. Restarting `agentvault serve` generates a new token.
- Treat the token like a local password: do not share it, commit it, or log it.

## CORS

The local API uses a restrictive CORS policy by default:

- Allowed origins are limited to local/browser-extension origins.
- Credentials are not exposed cross-origin.
- If you change the bind address or port, ensure the new endpoint is still restricted to trusted local clients.

## Browser extension

The browser extension is a Manifest V3 package:

- It communicates with `http://127.0.0.1:47321` (or the configured server URL).
- It requires the user-provided auth token for any write operation.
- It does not read credentials, cookies, or page content outside of the active tab when the user triggers a capture.
- Only install the extension from a trusted release artifact or the Chrome Web Store once published.

## Data storage

- Markdown files are the source of truth and live on your filesystem.
- The SQLite index (`<vault>/.agentvault/agentvault.db`) is a rebuildable cache.
- Vault configuration (`<vault>/.agentvault/config.json`) may contain an AI provider API key; keep the `.agentvault` directory private.

## AI provider credentials

- Cloud AI providers require an API key.
- The key can be supplied via the `AGENTVAULT_API_KEY` environment variable or stored in the vault config.
- Do not commit API keys to version control.

## Desktop and mobile clients

- The desktop app runs the local API in-process via Wails; no external network port is opened by default.
- The mobile app is an Expo app that connects to the local API on the same network; use it only on trusted networks.

## Reporting security issues

If you discover a security issue, please open a private issue or email the maintainer before disclosing it publicly.
