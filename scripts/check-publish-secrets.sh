#!/usr/bin/env bash
# Check whether the GitHub Actions secrets required for store publishing
# are configured for this repository.
#
# Usage:
#   scripts/check-publish-secrets.sh [macos|windows|chrome|mobile|all]...
#
# When no arguments are given, all platforms are checked. Required secrets
# are shown in red when missing; optional secrets are shown in yellow.

set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RESET='\033[0m'

# secret name -> description
MACOS_REQUIRED=(
  "MACOS_CERTIFICATE:Base64-encoded Developer ID Application .p12"
  "MACOS_CERTIFICATE_PASSWORD:Password for the .p12 file"
)
MACOS_OPTIONAL=(
  "MACOS_NOTARIZATION_APPLE_ID:Apple ID email for notarization"
  "MACOS_NOTARIZATION_TEAM_ID:Apple Developer Team ID"
  "MACOS_NOTARIZATION_PASSWORD:App-specific password for the Apple ID"
  "MACOS_SIGN_IDENTITY:Codesign identity (default: Developer ID Application)"
)

WINDOWS_REQUIRED=(
  "WINDOWS_CERTIFICATE:Base64-encoded code signing .pfx"
  "WINDOWS_CERTIFICATE_PASSWORD:Password for the .pfx file"
)

CHROME_REQUIRED=(
  "CHROME_WEBSTORE_CLIENT_ID:Chrome Web Store API OAuth client ID"
  "CHROME_WEBSTORE_CLIENT_SECRET:Chrome Web Store API OAuth client secret"
  "CHROME_WEBSTORE_REFRESH_TOKEN:OAuth refresh token for the upload API"
  "CHROME_WEBSTORE_EXTENSION_ID:Extension ID in the Chrome Web Store"
)

MOBILE_REQUIRED=(
  "EXPO_TOKEN:Expo access token for EAS CLI"
)
MOBILE_OPTIONAL=(
  "ASC_API_KEY_PATH:App Store Connect API private key (.p8) contents"
  "ASC_ISSUER_ID:App Store Connect API key issuer ID"
  "ASC_KEY_ID:App Store Connect API key ID"
  "ASC_APP_ID:App Store Connect app ID"
  "EXPO_APPLE_TEAM_ID:Apple Developer Team ID"
  "GOOGLE_SERVICE_ACCOUNT_KEY:Google Play service account JSON key contents"
)

PLATFORMS=("macos" "windows" "chrome" "mobile")
CHECK=()

if [ $# -eq 0 ]; then
  CHECK=("${PLATFORMS[@]}")
else
  for arg in "$@"; do
    case "$arg" in
      all)
        CHECK=("${PLATFORMS[@]}")
        ;;
      macos|windows|chrome|mobile)
        CHECK+=("$arg")
        ;;
      *)
        echo "Unknown platform: $arg" >&2
        echo "Usage: $0 [macos|windows|chrome|mobile|all]..." >&2
        exit 2
        ;;
    esac
  done
fi

fetch_secrets() {
  if command -v gh >/dev/null 2>&1 && gh auth status >/dev/null 2>&1; then
    gh secret list --json name -q '.[].name' 2>/dev/null || true
  else
    echo ""
  fi
}

configured_secrets=$(fetch_secrets)

is_set() {
  local name="$1"
  if [ -z "$configured_secrets" ]; then
    return 1
  fi
  printf '%s\n' "$configured_secrets" | grep -qx "$name"
}

print_secret_line() {
  local name="$1" desc="$2"
  if is_set "$name"; then
    printf "  ${GREEN}✓${RESET} %-45s %s\n" "$name" "$desc"
  else
    printf "  ${RED}✗${RESET} %-45s %s\n" "$name" "$desc"
  fi
}

check_platform() {
  local platform="$1"
  local -n req="${platform^^}_REQUIRED"
  local -n opt="${platform^^}_OPTIONAL"
  local missing=0

  echo "--- $platform ---"
  for entry in "${req[@]}"; do
    local name="${entry%%:*}" desc="${entry#*:}"
    print_secret_line "$name" "$desc"
    if ! is_set "$name"; then
      missing=$((missing + 1))
    fi
  done
  for entry in "${opt[@]}"; do
    local name="${entry%%:*}" desc="${entry#*:}"
    if is_set "$name"; then
      printf "  ${GREEN}✓${RESET} %-45s %s (optional)\n" "$name" "$desc"
    else
      printf "  ${YELLOW}○${RESET} %-45s %s (optional)\n" "$name" "$desc"
    fi
  done

  if [ "$missing" -eq 0 ]; then
    printf "${GREEN}  $platform is ready.${RESET}\n\n"
    return 0
  else
    printf "${RED}  $platform gated: $missing required secret(s) missing.${RESET}\n\n"
    return 1
  fi
}

total_missing=0
for platform in "${CHECK[@]}"; do
  if ! check_platform "$platform"; then
    total_missing=1
  fi
done

if [ -z "$configured_secrets" ]; then
  echo "Install the GitHub CLI (gh) and authenticate to get a live secret list."
  echo "Until then, this script shows the required secret names only."
fi

exit "$total_missing"
