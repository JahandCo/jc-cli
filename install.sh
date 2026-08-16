#!/bin/sh
# Installs jc, the Jah and Co Studio developer CLI, as a standalone binary —
# no Node or Bun required on the machine running this script.
#
#   curl -fsSL https://raw.githubusercontent.com/JahandCo/jc-cli/main/install.sh | sh
#
# Override the install location with JC_INSTALL_DIR (defaults to ~/.jc/bin,
# matching the same ~/.jc directory jc itself stores credentials in).
set -eu

REPO="JahandCo/jc-cli"
INSTALL_DIR="${JC_INSTALL_DIR:-$HOME/.jc/bin}"

fail() {
  echo "jc install: $1" >&2
  exit 1
}

detect_os() {
  case "$(uname -s)" in
    Linux) echo linux ;;
    Darwin) echo darwin ;;
    *) fail "unsupported OS $(uname -s) — download a binary manually from https://github.com/${REPO}/releases" ;;
  esac
}

detect_arch() {
  case "$(uname -m)" in
    x86_64 | amd64) echo x64 ;;
    arm64 | aarch64) echo arm64 ;;
    *) fail "unsupported architecture $(uname -m) — download a binary manually from https://github.com/${REPO}/releases" ;;
  esac
}

OS="$(detect_os)"
ARCH="$(detect_arch)"
ASSET="jc-${OS}-${ARCH}"

echo "Looking up the latest jc release..."
LATEST_TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | head -n1 | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
[ -n "$LATEST_TAG" ] || fail "couldn't determine the latest release from the GitHub API"

BASE_URL="https://github.com/${REPO}/releases/download/${LATEST_TAG}"

echo "Installing jc ${LATEST_TAG} (${ASSET}) to ${INSTALL_DIR}..."
mkdir -p "$INSTALL_DIR"
TMP_FILE="$(mktemp)"
trap 'rm -f "$TMP_FILE"' EXIT

curl -fsSL "${BASE_URL}/${ASSET}" -o "$TMP_FILE" || fail "download failed — is ${ASSET} a published asset on ${LATEST_TAG}?"

EXPECTED_SUM=$(curl -fsSL "${BASE_URL}/SHA256SUMS" | grep "  ${ASSET}\$" | cut -d' ' -f1 || true)
if [ -n "$EXPECTED_SUM" ]; then
  if command -v sha256sum >/dev/null 2>&1; then
    ACTUAL_SUM=$(sha256sum "$TMP_FILE" | cut -d' ' -f1)
  else
    ACTUAL_SUM=$(shasum -a 256 "$TMP_FILE" | cut -d' ' -f1)
  fi
  [ "$EXPECTED_SUM" = "$ACTUAL_SUM" ] || fail "checksum mismatch for ${ASSET} — download may be corrupted or tampered with"
else
  echo "Warning: no checksum found for ${ASSET} in this release's SHA256SUMS — skipping verification." >&2
fi

mv "$TMP_FILE" "$INSTALL_DIR/jc"
chmod +x "$INSTALL_DIR/jc"
trap - EXIT

echo "Installed jc ${LATEST_TAG} -> ${INSTALL_DIR}/jc"

case ":${PATH}:" in
  *":${INSTALL_DIR}:"*)
    echo ""
    "${INSTALL_DIR}/jc" --version >/dev/null 2>&1 && echo "jc is ready — run 'jc signin' to get started."
    ;;
  *)
    SHELL_RC="$HOME/.profile"
    case "$(basename "${SHELL:-}")" in
      zsh) SHELL_RC="$HOME/.zshrc" ;;
      bash) SHELL_RC="$HOME/.bashrc" ;;
    esac
    printf '\nexport PATH="%s:$PATH"\n' "$INSTALL_DIR" >> "$SHELL_RC"
    echo ""
    echo "Added ${INSTALL_DIR} to PATH in ${SHELL_RC}. Restart your shell, or run:"
    echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
    echo "Then: jc signin"
    ;;
esac
