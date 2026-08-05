#!/bin/bash
set -euo pipefail

# curl -1sLf https://packages.jahandco.dev/jc-cli/install.sh | sudo -E bash
#
# Wrap everything in a block so a truncated download (curl dropping mid-
# stream) can't execute a partial script -- bash won't run this block
# until the closing brace has actually been received.
{
    # 1. Configuration
    # packages.jahandco.dev is a custom domain mapped straight to the
    # `releases` Cloudflare R2 bucket's root by Cloudflare -- not the raw
    # R2 S3 API endpoint (that one's only for uploading releases, see
    # .goreleaser.yaml). `releases` is a shared bucket across multiple Jah
    # and Co projects -- everything jc-cli publishes lives under its own
    # "jc-cli/" prefix within it, hence the path below. Override
    # DOWNLOAD_BASE_URL for local testing against a fake server.
    BASE_DOMAIN="${DOWNLOAD_BASE_URL:-https://packages.jahandco.dev}/jc-cli"

    # Set VERSION=0.1.1 (or v0.1.1) to pin a specific release -- otherwise
    # resolve the current one from latest.txt, a plain-text file goreleaser
    # writes fresh to jc-cli/latest.txt on every release (see
    # .goreleaser.yaml's before.hooks). goreleaser's own {{ .Version }}
    # (the upload path this URL is built from) never has a leading "v"
    # even though git tags do (v0.1.2 tags, "0.1.2" in the URL) -- accept a
    # caller-supplied VERSION with or without one and strip it either way.
    if [ -z "${VERSION:-}" ]; then
        VERSION="$(curl -fsSL "${BASE_DOMAIN}/latest.txt")"
        if [ -z "$VERSION" ]; then
            echo "couldn't resolve the latest jc version from ${BASE_DOMAIN}/latest.txt -- pass VERSION=x.y.z to pin one explicitly" >&2
            exit 1
        fi
    fi
    VERSION="${VERSION#v}"

    BASE_URL="${BASE_DOMAIN}/cli/${VERSION}"

    # 2. Detect OS + architecture, matching goreleaser's own archive
    # name_template exactly (.goreleaser.yaml): {ProjectName}_{TitleCaseOS}_
    # {Arch}.tar.gz, e.g. jc_Linux_x86_64.tar.gz, jc_Darwin_arm64.tar.gz.
    # Windows isn't handled here -- there's no bash/sudo on Windows to pipe
    # this into; see README.md's PowerShell instructions instead.
    case "$(uname -s)" in
        Linux)  OS="Linux" ;;
        Darwin) OS="Darwin" ;;
        *)
            echo "jc's install.sh only supports Linux and macOS -- on Windows, use the PowerShell instructions in jc-cli's README.md instead" >&2
            exit 1
            ;;
    esac

    case "$(uname -m)" in
        x86_64|amd64)   ARCH="x86_64" ;;
        aarch64|arm64)  ARCH="arm64" ;;
        *)
            echo "Unsupported architecture: $(uname -m)" >&2
            exit 1
            ;;
    esac

    FILENAME="jc_${OS}_${ARCH}.tar.gz"
    DOWNLOAD_URL="${BASE_URL}/${FILENAME}"
    CHECKSUMS_URL="${BASE_URL}/checksums.txt"

    # If using sudo (system-wide), use /usr/local/bin.
    # If running as standard user, use ~/.local/bin
    if [ "$EUID" -eq 0 ]; then
        INSTALL_DIR="/usr/local/bin"
    else
        INSTALL_DIR="$HOME/.local/bin"
    fi

    # 3. Setup temporary extraction directory
    TMP_DIR=$(mktemp -d)
    trap 'rm -rf "$TMP_DIR"' EXIT
    echo "Downloading jc ${VERSION} for ${OS}/${ARCH}..."

    # 4. Download and verify against goreleaser's own published
    # checksums.txt before extracting anything -- this pipes straight into
    # sudo, so a corrupted or tampered download should fail loudly, not
    # silently install.
    curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$FILENAME"
    curl -fsSL "$CHECKSUMS_URL" -o "$TMP_DIR/checksums.txt"

    EXPECTED_HASH=$(grep " ${FILENAME}\$" "$TMP_DIR/checksums.txt" | awk '{print $1}')
    if [ -z "$EXPECTED_HASH" ]; then
        echo "no checksum entry for ${FILENAME} in checksums.txt -- aborting" >&2
        exit 1
    fi

    # sha256sum is GNU coreutils (Linux); macOS ships shasum instead --
    # prefer whichever is actually present rather than assuming one.
    if command -v sha256sum >/dev/null 2>&1; then
        ACTUAL_HASH=$(sha256sum "$TMP_DIR/$FILENAME" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        ACTUAL_HASH=$(shasum -a 256 "$TMP_DIR/$FILENAME" | awk '{print $1}')
    else
        echo "neither sha256sum nor shasum is available -- can't verify the download, aborting" >&2
        exit 1
    fi

    if [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
        echo "checksum mismatch for ${FILENAME}: expected ${EXPECTED_HASH}, got ${ACTUAL_HASH} -- aborting" >&2
        exit 1
    fi

    # 5. Extract
    tar -xzf "$TMP_DIR/$FILENAME" -C "$TMP_DIR"

    # 6. Install the binary
    mkdir -p "$INSTALL_DIR"
    mv "$TMP_DIR/jc" "$INSTALL_DIR/jc"
    chmod +x "$INSTALL_DIR/jc"

    # 7. Output completion message
    echo ""
    echo "jc ${VERSION} installed to ${INSTALL_DIR}/jc"
    case ":$PATH:" in
        *":$INSTALL_DIR:"*)
            echo "Run: jc login"
            ;;
        *)
            echo "${INSTALL_DIR} isn't on your PATH yet -- add it, then run: jc login"
            echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
            ;;
    esac

} # End of execution block
