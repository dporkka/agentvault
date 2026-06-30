#!/usr/bin/env bash
set -euo pipefail

REPO="agentvault/agentvault"
DEFAULT_INSTALL_DIR="${HOME}/.local/bin"
SYSTEM_INSTALL_DIR="/usr/local/bin"
GITHUB_DOWNLOAD_BASE="https://github.com/${REPO}/releases/download"

VERSION=""
INSTALL_DIR=""
FORCE=0
SYSTEM=0

usage() {
  cat <<EOF
Usage: $0 [options]

Options:
  --version <version>   Version to install (default: latest GitHub release)
  --install-dir <dir>   Directory to install the binary (default: \$HOME/.local/bin)
  --system              Install to /usr/local/bin
  --force               Overwrite an existing binary without prompting
  -h, --help            Show this help message
EOF
}

log_info() {
  printf '[install] %s\n' "$1"
}

log_error() {
  printf '[install] error: %s\n' "$1" >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --version)
      VERSION="${2:-}"
      if [ -z "$VERSION" ]; then
        log_error "--version requires a value"
        exit 1
      fi
      shift 2
      ;;
    --install-dir)
      INSTALL_DIR="${2:-}"
      if [ -z "$INSTALL_DIR" ]; then
        log_error "--install-dir requires a value"
        exit 1
      fi
      shift 2
      ;;
    --system)
      SYSTEM=1
      shift
      ;;
    --force)
      FORCE=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      log_error "Unknown option: $1"
      usage
      exit 1
      ;;
  esac
done

OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
  linux|darwin) ;;
  *)
    log_error "Unsupported operating system: ${OS}. Only linux and darwin are supported."
    exit 1
    ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)
    log_error "Unsupported architecture: ${ARCH}. Only amd64 and arm64 are supported."
    exit 1
    ;;
esac

if [ -z "$INSTALL_DIR" ]; then
  if [ "$SYSTEM" -eq 1 ]; then
    INSTALL_DIR="$SYSTEM_INSTALL_DIR"
  else
    INSTALL_DIR="$DEFAULT_INSTALL_DIR"
  fi
fi

if [ "$SYSTEM" -ne 1 ] && [ "$INSTALL_DIR" = "$DEFAULT_INSTALL_DIR" ]; then
  if [ -d "$INSTALL_DIR" ] && [ ! -w "$INSTALL_DIR" ]; then
    log_info "${INSTALL_DIR} is not writable, falling back to ${SYSTEM_INSTALL_DIR}"
    INSTALL_DIR="$SYSTEM_INSTALL_DIR"
  elif [ ! -d "$INSTALL_DIR" ]; then
    if ! mkdir -p "$INSTALL_DIR" 2>/dev/null; then
      log_info "Could not create ${INSTALL_DIR}, falling back to ${SYSTEM_INSTALL_DIR}"
      INSTALL_DIR="$SYSTEM_INSTALL_DIR"
    fi
  fi
fi

if [ ! -d "$INSTALL_DIR" ] && ! mkdir -p "$INSTALL_DIR"; then
  log_error "Could not create install directory: ${INSTALL_DIR}"
  exit 1
fi

if [ ! -w "$INSTALL_DIR" ]; then
  log_error "Install directory is not writable: ${INSTALL_DIR}"
  exit 1
fi

if [ -z "$VERSION" ]; then
  log_info "Determining latest release..."
  if ! VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/')"; then
    log_error "Could not determine latest release"
    exit 1
  fi
  if [ -z "$VERSION" ]; then
    log_error "Could not determine latest release"
    exit 1
  fi
fi

case "$VERSION" in
  v*) ;;
  *) VERSION="v${VERSION}" ;;
esac

log_info "Installing agentvault ${VERSION} for ${OS}_${ARCH} to ${INSTALL_DIR}"

TARGET_BIN="${INSTALL_DIR}/agentvault"
ARCHIVE_NAME="agentvault_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="${GITHUB_DOWNLOAD_BASE}/${VERSION}/${ARCHIVE_NAME}"

if [ -e "$TARGET_BIN" ] && [ "$FORCE" -ne 1 ]; then
  if [ ! -e /dev/tty ]; then
    log_error "${TARGET_BIN} already exists. Use --force to overwrite."
    exit 1
  fi
  printf '[install] %s already exists. Overwrite? [y/N] ' "$TARGET_BIN" >/dev/tty
  read -r response </dev/tty || true
  case "${response:-n}" in
    [yY][eE][sS]|[yY]) ;;
    *)
      log_info "Installation cancelled"
      exit 0
      ;;
  esac
fi

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

log_info "Downloading ${DOWNLOAD_URL}"
if ! curl -fsSL "$DOWNLOAD_URL" -o "${TMP_DIR}/${ARCHIVE_NAME}"; then
  log_error "Download failed: ${DOWNLOAD_URL}"
  exit 1
fi

log_info "Extracting archive..."
if ! tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "$TMP_DIR"; then
  log_error "Failed to extract archive"
  exit 1
fi

EXTRACTED_BIN="$(find "$TMP_DIR" -type f -name agentvault | head -n 1)"
if [ -z "$EXTRACTED_BIN" ]; then
  log_error "Could not find agentvault binary in archive"
  exit 1
fi

log_info "Installing binary to ${TARGET_BIN}"
if ! mv "$EXTRACTED_BIN" "$TARGET_BIN"; then
  log_error "Failed to install binary to ${TARGET_BIN}"
  exit 1
fi

if ! chmod +x "$TARGET_BIN"; then
  log_error "Failed to make binary executable"
  exit 1
fi

log_info "Verifying installation..."
if ! "$TARGET_BIN" version; then
  log_error "Installation verification failed"
  exit 1
fi

log_info "Successfully installed agentvault to ${TARGET_BIN}"

if ! command -v agentvault >/dev/null 2>&1; then
  log_info "Note: ${INSTALL_DIR} is not on your PATH. Add it to your shell profile:"
  printf '    export PATH="%s:$PATH"\n' "$INSTALL_DIR"
fi
