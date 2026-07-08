#!/usr/bin/env bash
# Smoke test for packaged AgentVault CLI artifacts.
# Builds the Linux CLI archive, extracts it, and exercises core commands.

set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "==> Building CLI archive..."
make release-cli-linux

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo "0.1.0")"
ARCHIVE="dist/cli/agentvault-${VERSION}-linux-amd64.tar.gz"

if [[ ! -f "$ARCHIVE" ]]; then
    echo "Archive not found: $ARCHIVE"
    exit 1
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

echo "==> Extracting archive..."
tar -xzf "$ARCHIVE" -C "$TMP_DIR"
BIN="$TMP_DIR/agentvault-linux-amd64"

if [[ ! -x "$BIN" ]]; then
    echo "Binary not found or not executable: $BIN"
    exit 1
fi

echo "==> Smoke testing packaged CLI..."

$BIN --version

VAULT="$TMP_DIR/smoke-vault"
$BIN init "$VAULT"

NOTE="$VAULT/10-notes/smoke.md"
mkdir -p "$(dirname "$NOTE")"
cat > "$NOTE" <<'EOF'
---
id: smoke-note
type: note
title: Smoke Test Note
---

This note is used by the CLI smoke test.
EOF

$BIN index --vault "$VAULT"
$BIN search "smoke" --vault "$VAULT"

PORT=$(python3 -c 'import socket; s=socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1]); s.close()')
HEALTH_URL="http://127.0.0.1:$PORT/health"

echo "==> Starting local API on port $PORT..."
$BIN serve --vault "$VAULT" --port "$PORT" &
SERVER_PID=$!
trap 'kill "$SERVER_PID" 2>/dev/null || true; rm -rf "$TMP_DIR"' EXIT

for _ in {1..30}; do
    if curl -fs "$HEALTH_URL" >/dev/null 2>&1; then
        break
    fi
    sleep 0.1
done

echo "==> Checking health endpoint..."
curl -fs "$HEALTH_URL" >/dev/null

echo "==> Smoke test passed."
