# Publishing Secrets Reference

How to obtain every secret needed for signed releases and store publishing.
All secrets go into GitHub → Settings → Secrets and variables → Actions.

Run `make check-secrets` to audit which are already configured.

## Desktop Signing

### macOS

| Secret | Source |
|---|---|
| `MACOS_CERTIFICATE` | Apple Developer account → Certificates, Identifiers & Profiles → **Developer ID Application** certificate → export as `.p12` → `base64 -i cert.p12` |
| `MACOS_CERTIFICATE_PASSWORD` | Password set during `.p12` export |
| `MACOS_NOTARIZATION_APPLE_ID` | Your Apple ID email |
| `MACOS_NOTARIZATION_TEAM_ID` | [developer.apple.com/account](https://developer.apple.com/account) → Membership |
| `MACOS_NOTARIZATION_PASSWORD` | [appleid.apple.com](https://appleid.apple.com) → App-Specific Passwords |
| `MACOS_SIGN_IDENTITY` | _(optional)_ Common name override. Default: `Developer ID Application`. Find yours: `security find-identity -v` |

Prerequisite: [Apple Developer Program](https://developer.apple.com/programs/) enrollment ($99/year).

### Windows

| Secret | Source |
|---|---|
| `WINDOWS_CERTIFICATE` | Code signing certificate (`.pfx`) from DigiCert, Sectigo, or [Azure Trusted Signing](https://learn.microsoft.com/en-us/azure/security/trusted-signing/). Encode: `[Convert]::ToBase64String([IO.File]::ReadAllBytes("cert.pfx"))` |
| `WINDOWS_CERTIFICATE_PASSWORD` | Password for the `.pfx` |

### Linux

No code signing. The release workflow produces a raw binary and tar.gz.

## Browser Extension

| Secret | Source |
|---|---|
| `CHROME_WEBSTORE_EXTENSION_ID` | [Chrome Web Store Developer Dashboard](https://chrome.google.com/webstore/devconsole/) → create extension item → copy the 32-char ID from the URL |
| `CHROME_WEBSTORE_CLIENT_ID` | Google Cloud Console → APIs & Services → Credentials → Create OAuth 2.0 Client ID (Desktop app) |
| `CHROME_WEBSTORE_CLIENT_SECRET` | Same OAuth credential |
| `CHROME_WEBSTORE_REFRESH_TOKEN` | Generate via [`chrome-webstore-upload`](https://github.com/fregante/chrome-webstore-upload) CLI using the client ID + secret above |

Prerequisite: one-time $5 [Chrome Web Store developer registration](https://chrome.google.com/webstore/devconsole/register).

## Mobile

### Expo

| Secret | Source |
|---|---|
| `EXPO_TOKEN` | [expo.dev](https://expo.dev) → Settings → Access Tokens → Create |

### Apple App Store

| Secret | Source |
|---|---|
| `ASC_API_KEY_PATH` | [App Store Connect](https://appstoreconnect.apple.com) → Users and Access → Keys → App Store Connect API → generate key → **copy the entire `.p8` file contents** (not a path) |
| `ASC_KEY_ID` | Same key page → Key ID |
| `ASC_ISSUER_ID` | Same key page → Issuer ID |
| `ASC_APP_ID` | _(optional)_ App Store Connect → App → App Information → Apple ID number. Lets EAS skip app creation. |
| `EXPO_APPLE_TEAM_ID` | _(optional)_ [developer.apple.com/account](https://developer.apple.com/account) → Membership |

Prerequisite: [Apple Developer Program](https://developer.apple.com/programs/) enrollment.

### Google Play Store

| Secret | Source |
|---|---|
| `GOOGLE_SERVICE_ACCOUNT_KEY` | Google Cloud Console → IAM → Service Accounts → create one → grant Release Manager in [Google Play Console](https://play.google.com/console) → download JSON key → **copy the entire JSON** |

Prerequisite: one-time $25 Google Play Developer registration.

## Checklist

```bash
# Audit current state
make check-secrets

# Check a single platform
make check-secrets macos
make check-secrets chrome
```

When a secret is missing, the workflow falls back to unsigned artifacts or bundle exports — it never fails the build.
