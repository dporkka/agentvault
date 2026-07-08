# AgentVault Compatibility Matrix

This document lists the supported platforms, toolchains, browsers, and AI providers for AgentVault.

## Operating systems

| Component | Linux | macOS | Windows |
| --- | --- | --- | --- |
| CLI | Ubuntu 24.04+, Debian 12+, Fedora 40+ | macOS 13+ (Intel/Apple Silicon) | Windows 10/11 |
| Desktop app | Ubuntu 24.04+/26.04 with GTK 3 and WebKit2GTK 4.1 | macOS 13+ | Windows 10/11 |
| Web app | Any modern browser | Any modern browser | Any modern browser |
| Browser extension | Chrome/Edge/Firefox on Linux | Chrome/Edge/Firefox on macOS | Chrome/Edge/Firefox on Windows |
| Mobile app | Android via Expo/EAS | iOS via Expo/EAS | — |

### Linux desktop requirements

The Wails desktop app requires the following system libraries:

```bash
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
```

The build tag `webkit2_41` is required on Ubuntu 24.04+ and 26.04 because these distributions ship WebKit2GTK 4.1 instead of 4.0.

## Toolchain versions

| Tool | Minimum version | Notes |
| --- | --- | --- |
| Go (core CLI) | 1.23 | Used by `core/` |
| Go (desktop app) | 1.25 | Used by `apps/desktop-wails/` |
| Node.js | 20 | Used by all frontend/mobile packages |
| npm | 10 | Used by all frontend/mobile packages |
| Wails CLI | v2.9.2 | Used to build the desktop app |
| Vite | 8 | Used by `apps/web-local` and `apps/browser-extension` |
| Expo SDK | ~56.0 | Used by `apps/mobile-expo` |
| React Native | 0.85 | Used by `apps/mobile-expo` |
| React (web/extension/desktop) | 18.3 | Used by `apps/web-local`, `apps/browser-extension`, `apps/desktop-wails/frontend` |
| React (mobile) | 19.2 | Used by `apps/mobile-expo` |
| Tailwind CSS | 3.4 | Used by all frontend clients |
| SQLite | FTS5 enabled | Provided by `modernc.org/sqlite` |

## Browsers

The browser extension and web app are tested against the latest stable versions of:

- Google Chrome
- Microsoft Edge
- Mozilla Firefox

The extension is distributed as a Manifest V3 package.

## AI providers

AgentVault can use any of the following providers for chat and embeddings:

| Provider | Base URL / setup | Embeddings | Notes |
| --- | --- | --- | --- |
| Ollama (default) | `http://localhost:11434` | Yes | Local-first default; requires Ollama running locally |
| OpenAI-compatible | `https://api.openai.com/v1` or custom | Yes | Set `AGENTVAULT_API_KEY` or store key in vault config |
| Anthropic | `https://api.anthropic.com` | No | Chat only; API key required |
| OpenRouter | `https://openrouter.ai/api` | No | Chat only; API key required |
| Mock | `mock` | No | For tests; returns deterministic responses |

Cloud providers read `AGENTVAULT_API_KEY` when no key is stored in the vault config.

## Architecture support

| Artifact | amd64 | arm64 | Notes |
| --- | --- | --- | --- |
| CLI Linux | ✓ | ✓ | Static binary |
| CLI macOS | ✓ | ✓ | Static binary |
| CLI Windows | ✓ | — | Static binary |
| Desktop Linux | ✓ | — | Requires GTK/WebKit2GTK |
| Desktop macOS | — | ✓ | Signed `.dmg` when secrets configured |
| Desktop Windows | ✓ | — | Signed NSIS installer when secrets configured |

## Reporting compatibility issues

If you encounter a problem on a supported configuration, please open an issue with the output of:

```bash
agentvault --version
uname -a        # Linux/macOS
ver             # Windows
```
