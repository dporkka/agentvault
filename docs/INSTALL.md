# Installing AgentVault

AgentVault is a local-first knowledge operating system. The core engine, CLI, and local HTTP API are packaged as a single Go binary. First-party clients include a Wails desktop app, a React web app, a Manifest V3 browser extension, and an Expo mobile app.

## Table of contents

- [CLI](#cli)
- [Desktop app](#desktop-app)
- [Web app](#web-app)
- [Browser extension](#browser-extension)
- [Mobile app](#mobile-app)
- [Building from source](#building-from-source)

## CLI

The CLI is a single static binary. Download the archive for your platform from the [GitHub Releases](https://github.com/dporkka/agentvault/releases) page and extract it.

### Linux

```bash
# Replace <version> and <arch> with the release you want, e.g. v0.1.0 amd64
curl -LO https://github.com/dporkka/agentvault/releases/download/<version>/agentvault-<version>-linux-<arch>.tar.gz
tar -xzf agentvault-<version>-linux-<arch>.tar.gz
sudo mv agentvault-linux-<arch> /usr/local/bin/agentvault
agentvault --version
```

### macOS

```bash
curl -LO https://github.com/dporkka/agentvault/releases/download/<version>/agentvault-<version>-darwin-<arch>.tar.gz
tar -xzf agentvault-<version>-darwin-<arch>.tar.gz
sudo mv agentvault-darwin-<arch> /usr/local/bin/agentvault
agentvault --version
```

### Windows

Download `agentvault-<version>-windows-amd64.zip` from the releases page, extract it, and add the folder containing `agentvault-windows-amd64.exe` to your PATH. You can rename the executable to `agentvault.exe` for convenience.

### Verify the install

```bash
agentvault --help
agentvault init ./my-vault
```

## Desktop app

### Linux

A prebuilt Linux binary is attached to each release as `agentvault-desktop-linux-amd64`. It requires GTK 3 and WebKit2GTK 4.1.

On Ubuntu 24.04+ / 26.04:

```bash
sudo apt-get install -y libgtk-3-dev libwebkit2gtk-4.1-dev
chmod +x agentvault-desktop-linux-amd64
./agentvault-desktop-linux-amd64
```

### macOS and Windows

Prebuilt macOS `.app` bundles and Windows installers are not yet produced by CI. Build them from source with the Wails CLI:

```bash
cd apps/desktop-wails
wails build
```

See [Building from source](#building-from-source) for details.

## Web app

The web app is a static React/Vite client for the local HTTP API. It is included in releases as part of the source tree and can be built locally:

```bash
cd apps/web-local
npm ci
npm run build
```

The built files are in `apps/web-local/dist/` and can be served by any static file server after starting `agentvault serve`.

## Browser extension

Download `agentvault-extension-<version>.zip` from the releases page and load it unpacked:

1. Open Chrome/Edge/Firefox and navigate to the extensions page.
2. Enable developer mode.
3. Choose "Load unpacked" and select the extracted extension folder.

The extension connects to `agentvault serve` on `http://127.0.0.1:47321` (or the configured server URL) and requires the auth token printed at server startup.

## Mobile app

The mobile app is an Expo React Native project. There are two ways to install it:

### Expo prebuilt bundles

Each release includes `agentvault-mobile-<version>-ios.zip` and `agentvault-mobile-<version>-android.zip`. These are Metro export bundles suitable for Expo/EAS builds or over-the-air updates. They are not standalone installable apps.

### Build with EAS

To produce installable iOS/Android binaries, you need an Expo account and Apple/Google developer credentials:

```bash
cd apps/mobile-expo
npm ci
npx eas build --platform ios --profile production
npx eas build --platform android --profile production
```

See the [Expo documentation](https://docs.expo.dev/build/setup/) for credential setup.

## Building from source

### Prerequisites

- Go 1.23+ for the core CLI (`core/`)
- Go 1.25+ for the desktop app (`apps/desktop-wails/`)
- Node.js 20+ and npm
- Linux desktop builds: `libgtk-3-dev` and `libwebkit2gtk-4.1-dev`
- Wails CLI for desktop builds: `go install github.com/wailsapp/wails/v2/cmd/wails@latest`

### Build everything

```bash
make release
```

This produces all release artifacts in `dist/`:

```text
dist/
├── cli/
│   ├── agentvault-<version>-linux-amd64.tar.gz
│   ├── agentvault-<version>-linux-arm64.tar.gz
│   ├── agentvault-<version>-darwin-amd64.tar.gz
│   ├── agentvault-<version>-darwin-arm64.tar.gz
│   └── agentvault-<version>-windows-amd64.zip
├── desktop/
│   └── agentvault-desktop-linux-amd64
├── extension/
│   └── agentvault-extension-<version>.zip
└── mobile/
    ├── agentvault-mobile-<version>-ios.zip
    └── agentvault-mobile-<version>-android.zip
```

### Build individual artifacts

```bash
make release-cli           # CLI archives for all platforms
make release-desktop-linux # Linux desktop binary
make release-extension     # Browser extension zip
make release-mobile        # Mobile export bundles
```

### Run tests and checks

```bash
make ci
```

This runs the same checks as CI: Go lint, Go tests with race detection, and the shared API contract check.
