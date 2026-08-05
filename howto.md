# Shipping the `jc` CLI

Instructions for releasing and distributing new versions of the `jc` CLI.

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
git tag -a v0.1.3 -m "jc CLI v0.1.3 -- release summary"
git push origin v0.1.3
```

### What happens automatically:
* **GitHub Actions Trigger**: The `v*` tag push automatically triggers the release workflow in `.github/workflows/release.yml`.
* **GoReleaser**: Cross-compiles `jc` for Linux (`amd64`, `arm64`), macOS (`amd64`, `arm64`), and Windows (`amd64`).
* **Artifact Upload**: Archives and `checksums.txt` are uploaded via GoReleaser's S3 provider directly to the self-hosted MinIO `cli-releases` bucket and made available immediately for download.

---

## Alternative Release Options (Terminal / Manual)

### Option 1: Live Release Upload from Terminal (GoReleaser)
Once you have committed your changes and created a local tag:

```bash
cd jc-cli

# Requires CLI_RELEASES_S3_BUCKET, CLI_RELEASES_S3_ENDPOINT, and MinIO
# credentials (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY) for the
# cli-releases bucket exported in your shell -- pull these from wherever
# your own secrets store keeps them, not committed anywhere in this repo.

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
  -e CLI_RELEASES_S3_BUCKET="cli-releases" \
  -e CLI_RELEASES_S3_ENDPOINT="https://api-s3.jahandco.net" \
  -e AWS_ACCESS_KEY_ID="$AWS_ACCESS_KEY_ID" \
  -e AWS_SECRET_ACCESS_KEY="$AWS_SECRET_ACCESS_KEY" \
  jc-goreleaser release --clean
```

---

## Installing the `jc` CLI

You can download and install any release directly from the S3 distribution storage:

### Linux (x86_64)
```bash
curl -fsSL https://api-s3.jahandco.net/cli-releases/cli/0.1.2/jc_Linux_x86_64.tar.gz | sudo tar -xz -C /usr/local/bin
```
*(For ARM64 Linux: `jc_Linux_arm64.tar.gz`)*

### macOS (Apple Silicon - M1/M2/M3)
```bash
curl -fsSL https://api-s3.jahandco.net/cli-releases/cli/0.1.2/jc_Darwin_arm64.tar.gz | sudo tar -xz -C /usr/local/bin
```
*(For Intel Mac: `jc_Darwin_x86_64.tar.gz`)*

### Windows (PowerShell)
```powershell
Invoke-WebRequest -Uri "https://api-s3.jahandco.net/cli-releases/cli/0.1.2/jc_Windows_x86_64.zip" -OutFile "$env:TEMP\jc.zip"
Expand-Archive -Path "$env:TEMP\jc.zip" -DestinationPath "$env:LOCALAPPDATA\jc" -Force
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:LOCALAPPDATA\jc", "User")
```

### Local Dev Build (from source)
```bash
cd jc-cli
go install .
```
