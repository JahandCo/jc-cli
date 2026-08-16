# jc-cli

Standalone binary releases of `jc`, the Jah and Co Studio developer CLI —
the `jc` command developers use to build titles against `@jahandco/game-sdk`
(scaffold, run locally, and upload to Developer Studio).

This repo exists only to host public, install-able binaries. `jc`'s actual
source lives in the (private) `jc-development` monorepo at
`packages/cli/` — this repo doesn't contain any of it, just the compiled
release artifacts and the install script.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/JahandCo/jc-cli/main/install.sh | sh
```

Detects your OS/arch, downloads the current release from this repo,
verifies it against the published `SHA256SUMS`, and installs to `~/.jc/bin`
(override with `JC_INSTALL_DIR`).

Supported platforms: Linux (x64, arm64), macOS (x64, arm64), Windows (x64 —
download the `.exe` asset directly from [Releases](../../releases/latest)
for now; the script above targets Linux/macOS).

`jc` is a standalone binary — no Node.js or Bun installation required.

## Usage

```bash
jc signin      # authenticate with Developer Studio
jc game init   # scaffold a title, linked to a Developer Studio project
jc dev         # run it locally
jc build       # build for production
jc upload      # build + upload to Developer Studio
```

Run `jc --help` for the full command list.
