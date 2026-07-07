# Publishing AgentVault

This document lists the GitHub secrets, accounts, and one-time setup required to produce signed desktop installers and publish the browser extension and mobile apps to their respective stores.

All publishing workflows are **secret-gated**: they build and upload unsigned/test artifacts when credentials are absent, and only sign or publish when the required secrets are configured.

## Table of contents

- [GitHub release workflow](#github-release-workflow)
- [Desktop signing](#desktop-signing)
  - [macOS](#macos)
  - [Windows](#windows)
- [Browser extension](#browser-extension)
- [Mobile apps](#mobile-apps)
  - [Expo account](#expo-account)
  - [Apple App Store](#apple-app-store)
  - [Google Play Store](#google-play-store)

## GitHub release workflow

The [`.github/workflows/release.yml`](../.github/workflows/release.yml) workflow runs on every `v*.*.*` tag and creates a draft GitHub release with the following artifacts:

- CLI archives for Linux, macOS, and Windows (`dist/cli/`)
- Linux desktop binary and tar.gz (`dist/desktop/`)
- macOS `.app` bundle and `.dmg` (`dist/macos/`)
- Windows installer `.exe` (`dist/windows/`)
- Browser extension zip (`dist/extension/`)
- Mobile iOS and Android export zips (`dist/mobile/`)

Trigger a release by pushing a tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

## Desktop signing

### macOS

Required secrets:

| Secret | Description |
| --- | --- |
| `MACOS_CERTIFICATE` | Base64-encoded Apple Developer ID Application certificate (`.p12`). |
| `MACOS_CERTIFICATE_PASSWORD` | Password for the `.p12` file. |
| `MACOS_NOTARIZATION_APPLE_ID` | Apple ID email for notarization. |
| `MACOS_NOTARIZATION_TEAM_ID` | Apple Developer Team ID. |
| `MACOS_NOTARIZATION_PASSWORD` | App-specific password for the Apple ID. |

Setup:

1. Enroll in the [Apple Developer Program](https://developer.apple.com/programs/).
2. Create a **Developer ID Application** certificate in Apple Developer Certificates, Identifiers & Profiles.
3. Export the certificate as a `.p12` file and base64-encode it:
   ```bash
   base64 -i DeveloperIDApplication.p12 | pbcopy
   ```
4. Create an app-specific password for your Apple ID.
5. Add the secrets to the GitHub repository under Settings > Secrets and variables > Actions.

When `MACOS_CERTIFICATE` is absent, the workflow produces an unsigned `.app` and `AgentVault-unsigned.dmg`. When present, it signs the `.app`, creates `AgentVault.dmg`, and notarizes it if notarization secrets are also present.

### Windows

Required secrets:

| Secret | Description |
| --- | --- |
| `WINDOWS_CERTIFICATE` | Base64-encoded code signing certificate (`.pfx`). |
| `WINDOWS_CERTIFICATE_PASSWORD` | Password for the `.pfx` file. |

Setup:

1. Purchase a code signing certificate from a trusted CA (e.g., DigiCert, Sectigo, Certum) or set up [Azure Trusted Signing](https://learn.microsoft.com/en-us/azure/security/trusted-signing/).
2. Export the certificate as a `.pfx` file and base64-encode it:
   ```powershell
   [Convert]::ToBase64String([IO.File]::ReadAllBytes("certificate.pfx")) | Set-Clipboard
   ```
3. Add the secrets to the GitHub repository.

When `WINDOWS_CERTIFICATE` is absent, the workflow produces an unsigned NSIS installer `.exe`. When present, it signs all `.exe` files in the build output.

### Linux

Linux does not use code signing in the same way as macOS/Windows. The release workflow produces a raw binary and a tar.gz archive. You may distribute the binary directly or package it as a `.deb`, `.rpm`, or AppImage separately.

## Browser extension

Workflow: [`.github/workflows/publish-extension.yml`](../.github/workflows/publish-extension.yml)

Required secrets:

| Secret | Description |
| --- | --- |
| `CHROME_WEBSTORE_CLIENT_ID` | Chrome Web Store API OAuth client ID. |
| `CHROME_WEBSTORE_CLIENT_SECRET` | Chrome Web Store API OAuth client secret. |
| `CHROME_WEBSTORE_REFRESH_TOKEN` | OAuth refresh token for the upload API. |
| `CHROME_WEBSTORE_EXTENSION_ID` | The extension ID in the Chrome Web Store. |

Setup:

1. Register as a [Chrome Web Store developer](https://chrome.google.com/webstore/devconsole/).
2. Create the extension item in the developer dashboard to obtain the extension ID.
3. Enable the Chrome Web Store API in the Google Cloud Console and create OAuth credentials.
4. Follow the [chrome-webstore-upload](https://github.com/fregante/chrome-webstore-upload) documentation to generate a refresh token.
5. Add the secrets to the GitHub repository.

The workflow runs on tags and via `workflow_dispatch`. If the secrets are absent, it uploads the extension zip as a workflow artifact instead of publishing.

## Mobile apps

Workflow: [`.github/workflows/publish-mobile.yml`](../.github/workflows/publish-mobile.yml)

### Expo account

Required secret:

| Secret | Description |
| --- | --- |
| `EXPO_TOKEN` | Expo access token for EAS CLI. |

Setup:

1. Create an [Expo account](https://expo.dev/signup).
2. Generate an access token at https://expo.dev/settings/access-tokens.
3. Add `EXPO_TOKEN` to the GitHub repository secrets.

When `EXPO_TOKEN` is absent, the workflow exports local iOS/Android bundles and uploads them as artifacts instead of running EAS builds.

### Apple App Store

Required secrets (in addition to `EXPO_TOKEN`):

| Secret | Description |
| --- | --- |
| `ASC_API_KEY_PATH` | Path or content of the App Store Connect API private key (`.p8`). |
| `ASC_ISSUER_ID` | App Store Connect API key issuer ID. |
| `ASC_KEY_ID` | App Store Connect API key ID. |

Setup:

1. Enroll in the [Apple Developer Program](https://developer.apple.com/programs/).
2. Generate an App Store Connect API key in App Store Connect > Users and Access > Keys.
3. Configure the app bundle identifier and provisioning profile in Apple Developer.
4. Add the secrets to the GitHub repository and reference them in `eas.json` if needed.

When these secrets are present, the workflow runs `eas build --platform ios --profile production --auto-submit`.

### Google Play Store

Required secrets (in addition to `EXPO_TOKEN`):

| Secret | Description |
| --- | --- |
| `GOOGLE_SERVICE_ACCOUNT_KEY` | JSON key for a Google Play service account. |

Setup:

1. Create a Google Play Developer account.
2. Set up a service account in the Google Cloud Console and grant it Release Manager access in Google Play Console.
3. Download the service account JSON key and add it as the `GOOGLE_SERVICE_ACCOUNT_KEY` secret.

When this secret is present, the workflow runs `eas build --platform android --profile production --auto-submit`.

## Running publish workflows manually

You can trigger the extension and mobile workflows manually from the GitHub Actions tab. Each workflow has a `publish_to_store` / `submit_to_stores` input that is `false` by default; set it to `true` only when the required secrets are configured and you want to publish.
