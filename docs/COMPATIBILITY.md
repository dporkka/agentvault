# AgentVault Compatibility

This page lists the supported language runtimes, operating systems, browsers, and AI providers.

## Runtimes

| Component | Minimum version | Notes |
| --- | --- | --- |
| Go | 1.23 | Required for the core engine and CLI. |
| Node.js | 18+ (20+ recommended) | Required for the web app, browser extension, and desktop frontend builds. |

## Operating system support

| Component | Linux | macOS | Windows | Notes |
| --- | --- | --- | --- | --- |
| CLI | Yes | Yes | Yes | Cross-compiled releases for amd64 and arm64. |
| Desktop app | Yes | Yes | Yes | Built with Wails v2. Ubuntu 24.04+ requires the `webkit2_41` build tag. |
| Mobile app | No | No | No | iOS and Android are experimental via Expo; desktop OS support is not applicable. |

## Browser extension support

| Browser | Status | Notes |
| --- | --- | --- |
| Chrome | Supported | Manifest V3. |
| Edge | Supported | Manifest V3. |
| Firefox | Supported | Manifest V3 where available. |

## Mobile support

| Platform | Status | Notes |
| --- | --- | --- |
| iOS | Experimental | Via Expo. |
| Android | Experimental | Via Expo. |

## AI providers

| Provider | Status | Notes |
| --- | --- | --- |
| Ollama | Supported | Default provider; runs locally. |
| OpenAI | Supported | Cloud provider; requires API key. |
| Anthropic | Supported | Cloud provider; requires API key. |
| OpenRouter | Supported | Cloud provider; requires API key. |
| Mock | Supported | Returns static responses for testing. |

## SQLite driver

The CLI and desktop app use `modernc.org/sqlite`, a pure-Go SQLite driver with FTS5 support. It does not require CGO or a system SQLite installation, which simplifies cross-compilation.
