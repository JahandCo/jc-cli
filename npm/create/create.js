#!/usr/bin/env node
"use strict";

// npx @jahandco/create@latest [name] -- the zero-install entry point for
// trying the SDK. Deliberately thin: this script owns no scaffolding
// templates and no interactive prompts of its own. Its only two jobs are
// (1) make sure the real `jc` binary is on PATH, downloading it from the
// same Cloudflare R2 location `jc-cli`'s own GoReleaser config uploads to
// if it isn't, and (2) exec `jc init title <args> --local`, which does all
// the actual scaffolding and prompting. Every interactive prompt a user
// sees comes from jc itself (see jc-cli's cmd/init.go) -- duplicating that
// here would create exactly the kind of drift this whole workspace's own
// CLAUDE.md already warns about for shared/ui and shared/clerkuser.
//
// `jc init title --local` (not the default, backend-talking `jc init
// title`) skips jc login/project creation entirely -- linking the
// scaffolded directory to a real project is a separate, explicit step
// (`jc project link`) done afterward, once the developer actually wants to
// deploy/develop against the platform.

const { spawnSync } = require("node:child_process");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");

// Releases live in Cloudflare R2's `releases` bucket, but downloads go
// through this custom domain (mapped straight to the bucket's root by
// Cloudflare, not the raw R2 S3 API endpoint used to *upload* releases --
// see jc-cli's own .goreleaser.yaml). `releases` is a shared bucket across
// multiple Jah and Co projects -- everything jc-cli publishes lives under
// its own "jc-cli/" prefix within it, hence the path below.
const DOWNLOAD_BASE_URL = (process.env.DOWNLOAD_BASE_URL || "https://packages.jahandco.dev") + "/jc-cli";

// Set JC_VERSION=0.1.1 to pin a specific release -- otherwise resolved at
// runtime from latest.txt, a plain-text file goreleaser writes fresh to
// the bucket root on every release (see jc-cli's own .goreleaser.yaml
// before.hooks). No leading "v" either way -- goreleaser's own
// {{ .Version }} (what the upload path is built from) never has one.
async function resolveVersion() {
  if (process.env.JC_VERSION) {
    return process.env.JC_VERSION.replace(/^v/, "");
  }
  const res = await fetch(`${DOWNLOAD_BASE_URL}/latest.txt`);
  if (!res.ok) {
    throw new Error(`failed to resolve the latest jc version from ${DOWNLOAD_BASE_URL}/latest.txt: ${res.status} ${res.statusText}`);
  }
  const version = (await res.text()).trim();
  if (!version) {
    throw new Error(`${DOWNLOAD_BASE_URL}/latest.txt was empty -- set JC_VERSION to pin a release explicitly`);
  }
  return version;
}

function jcHomeBinDir() {
  return path.join(os.homedir(), ".jahandco", "bin");
}

function jcBinaryName() {
  return process.platform === "win32" ? "jc.exe" : "jc";
}

// Mirrors jc-cli's .goreleaser.yaml archive name_template exactly:
// {ProjectName}_{TitleCaseOS}_{Arch}.{tar.gz|zip}, e.g.
// jc_Linux_x86_64.tar.gz, jc_Darwin_arm64.tar.gz, jc_Windows_x86_64.zip.
// windows/arm64 is excluded from goreleaser's own build matrix -- there is
// no such archive to fetch.
function resolveArchive() {
  const osNames = { linux: "Linux", darwin: "Darwin", win32: "Windows" };
  const osName = osNames[process.platform];
  if (!osName) {
    throw new Error(
      `unsupported platform "${process.platform}" -- install jc manually, visit https://jahandco.dev/developer/cli`,
    );
  }

  if (process.platform === "win32" && process.arch === "arm64") {
    throw new Error("jc has no windows/arm64 build -- install manually (visit https://jahandco.dev/developer/cli)");
  }

  const archNames = { x64: "x86_64", arm64: "arm64" };
  const archName = archNames[process.arch];
  if (!archName) {
    throw new Error(`unsupported architecture "${process.arch}" -- install jc manually, visit https://jahandco.dev/developer/cli`);
  }

  const ext = process.platform === "win32" ? "zip" : "tar.gz";
  return `jc_${osName}_${archName}.${ext}`;
}

// A successful spawn (even a nonzero exit -- cobra prints usage and exits
// non-0 for a bare `jc` with no subcommand) means the binary exists.
// result.error is only set when the OS couldn't find/exec it at all.
function isJcOnPath() {
  const result = spawnSync("jc", [], { stdio: "ignore" });
  return !result.error;
}

function ensureTarAvailable() {
  const result = spawnSync("tar", ["--version"], { stdio: "ignore" });
  if (result.error) {
    throw new Error(
      "no `tar` command found on this system -- needed to extract jc's release archive (Windows 10 1803+ ships one as tar.exe, and it also handles the .zip format jc ships for windows). Install jc manually instead, see jc-cli's howto.md",
    );
  }
}

async function downloadAndInstallJc() {
  ensureTarAvailable();

  const version = await resolveVersion();
  const archiveName = resolveArchive();
  const url = `${DOWNLOAD_BASE_URL}/cli/${version}/${archiveName}`;

  console.log(`[create] jc not found on PATH -- downloading ${url}`);
  const res = await fetch(url);
  if (!res.ok) {
    throw new Error(`failed to download ${url}: ${res.status} ${res.statusText}`);
  }
  const bytes = Buffer.from(await res.arrayBuffer());

  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "jahandco-create-"));
  const archivePath = path.join(tmpDir, archiveName);
  fs.writeFileSync(archivePath, bytes);

  const binDir = jcHomeBinDir();
  fs.mkdirSync(binDir, { recursive: true });

  // `tar -xf` auto-detects the archive format from content, not just the
  // extension -- this works for the .zip windows ships too (bsdtar, which
  // both macOS's and modern Windows' bundled `tar` are), so one extraction
  // command covers all three platforms without an npm dependency.
  const extract = spawnSync("tar", ["-xf", archivePath, "-C", binDir], { stdio: "inherit" });
  if (extract.status !== 0) {
    throw new Error(`failed to extract ${archivePath} into ${binDir}`);
  }

  const binPath = path.join(binDir, jcBinaryName());
  if (process.platform !== "win32") {
    fs.chmodSync(binPath, 0o755);
  }

  console.log(`[create] installed jc to ${binPath}`);

  const onRealPath = (process.env.PATH || "").split(path.delimiter).some((entry) => path.resolve(entry || ".") === path.resolve(binDir));
  if (!onRealPath) {
    console.log(`[create] note: add ${binDir} to your PATH to run \`jc\` directly next time, e.g.:`);
    console.log(`  export PATH="${binDir}:$PATH"`);
  }
}

function execJcInitLocal(args) {
  const binDir = jcHomeBinDir();
  const env = { ...process.env };
  if (fs.existsSync(path.join(binDir, jcBinaryName()))) {
    env.PATH = `${binDir}${path.delimiter}${env.PATH || ""}`;
  }

  const result = spawnSync("jc", ["init", "title", ...args, "--local"], { stdio: "inherit", env });
  if (result.error) {
    throw result.error;
  }

  if (result.status === 0) {
    console.log("[create] next: jc login, then jc project link to connect this directory to a project");
  }
  process.exitCode = result.status ?? 1;
}

async function main() {
  const args = process.argv.slice(2);

  if (!isJcOnPath()) {
    await downloadAndInstallJc();
  }

  execJcInitLocal(args);
}

main().catch((err) => {
  console.error(`[create] ${err.message || err}`);
  process.exitCode = 1;
});
