# Shipping the JC CLI

Instructions for releasing and distributing new versions of the Jah and Co
Developer CLI (`jc`, the command it installs).

---

## Standard Release Flow (CI Automated)

### 1. Commit and push your changes
Ensure your working tree is clean and changes are pushed to `main`:

```bash
git add .
git commit -m "feat(cli): your release notes"
git push origin main
```

### 2. Tag a new version and push the tag
Create an annotated git tag matching `v*` (e.g. `v0.1.3`) and push it to origin:

```bash
git tag -a v0.1.3 -m "JC CLI v0.1.3 -- release summary"
git push origin v0.1.3
```

### What happens automatically:
* **GitHub Actions Trigger**: The `v*` tag push automatically triggers the release workflow in `.github/workflows/release.yml`.
* **GoReleaser**: Cross-compiles `jc` for Linux (`amd64`, `arm64`), macOS (`amd64`, `arm64`), and Windows (`amd64`).
* **Artifact Upload**: Archives and `checksums.txt` are uploaded via GoReleaser's S3 provider to the shared `releases` bucket on Cloudflare R2, under this project's own `jc-cli/` prefix, and made available immediately for download at `packages.jahandco.dev/jc-cli/` (a custom domain mapped straight to the bucket's root).

---

## Alternative Release Options (Terminal / Manual)

### Option 1: Live Release Upload from Terminal (GoReleaser)
Once you have committed your changes and created a local tag:

```bash
cd jc-cli

# Requires CLI_RELEASES_S3_BUCKET, CLI_RELEASES_S3_ENDPOINT, and R2 API
# token credentials (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY -- an R2
# API token's id/secret, from Cloudflare dashboard -> R2 -> Manage API
# Tokens) for the releases bucket exported in your shell -- pull these
# from wherever your own secrets store keeps them, not committed anywhere
# in this repo.

goreleaser release --clean
```

---

### Option 2: Test / Build Locally without Uploading (Snapshot Mode)
To cross-compile all platform binaries (Linux `amd64`/`arm64`, macOS `amd64`/`arm64`, Windows `amd64`), create archives, and generate `checksums.txt` in `dist/` without uploading:

```bash
cd jc-cli
goreleaser release --snapshot --clean
```

---

### Option 3: Using the Docker Release Image
To run the exact container build environment used by CI:

```bash
cd jc-cli
docker build -f Dockerfile.goreleaser -t jc-goreleaser .

docker run --rm \
  -e CLI_RELEASES_S3_BUCKET="releases" \
  -e CLI_RELEASES_S3_ENDPOINT="https://90bb3d25c7129244bd604db183d93c1c.r2.cloudflarestorage.com" \
  -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" \
  -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY" \
  jc-goreleaser release --clean
```

---

## Installing `jc`

You can download and install any release directly from
`packages.jahandco.dev`, a custom domain mapped straight to the shared
`releases` R2 bucket's root -- everything jc-cli publishes lives under its
own `jc-cli/` prefix within it, so URLs below start with `/jc-cli/`
(unlike the raw R2 S3 API endpoint, which uses the bucket name instead):

### Script (recommended, Linux/macOS)
```bash
curl -1sLf https://packages.jahandco.dev/jc-cli/install.sh | sudo -E bash
```

### Linux (x86_64, manual)
```bash
curl -fsSL https://packages.jahandco.dev/jc-cli/cli/0.1.2/jc_Linux_x86_64.tar.gz | sudo tar -xz -C /usr/local/bin
```
*(For ARM64 Linux: `jc_Linux_arm64.tar.gz`)*

### macOS (Apple Silicon - M1/M2/M3, manual)
```bash
curl -fsSL https://packages.jahandco.dev/jc-cli/cli/0.1.2/jc_Darwin_arm64.tar.gz | sudo tar -xz -C /usr/local/bin
```
*(For Intel Mac: `jc_Darwin_x86_64.tar.gz`)*

### Windows (PowerShell)
```powershell
Invoke-WebRequest -Uri "https://packages.jahandco.dev/jc-cli/cli/0.1.2/jc_Windows_x86_64.zip" -OutFile "$env:TEMP\jc.zip"
Expand-Archive -Path "$env:TEMP\jc.zip" -DestinationPath "$env:LOCALAPPDATA\jc" -Force
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:LOCALAPPDATA\jc", "User")
```

### Local Dev Build (from source)
```bash
cd jc-cli
go install .
```
