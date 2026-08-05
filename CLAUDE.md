# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

The **Jah and Co Developer CLI** ("**JC CLI**" for short) is the developer
CLI for the Jah and Co Developer Platform (`jahandco.dev`) — `jc` is the
command it installs: `jc login`, `jc project`, `jc init title`, `jc dev`,
`jc deploy`, `jc assets`, `jc console`. It split out of `developer-platform`
on 2026-08-04, where it lived as `services/cli` — a plain Go module with no
`package.json`, not wired into that repo's pnpm/Turborepo pipeline. See
`developer-platform`'s own `CLAUDE.md` for the history.

`jc init` scaffolds a new title against the [Game SDK](https://github.com/JahandCo/game-sdk)
(`@jahandco/game-sdk`/`@jahandco/interactive-protocol` — a separate repo,
consumed as published npm dependencies, never a workspace link). **On
2026-08-05 the scaffold templates were rewritten** to target `game-sdk`'s
own ground-up IoC rewrite (`@jahandco/game-sdk`, replacing
`@jahandco/interactive-sdk` — see that repo's `CLAUDE.md`/`sdk-changelog.md`
for the full rationale): a scaffolded title's `src/client.ts` is now just
`Engine.launch({...})` — no React shell, no hand-built `Phaser.Game`
config, no manual bridge/plugin wiring, no `Boot`/`MainMenu` scenes (Engine
owns the boot sequence, including a built-in default lobby, itself). The
build tool also changed in the same pass: scaffolded projects use
**Webpack** now, not esbuild, for both the client bundle
(`webpack.client.js`) and the server-side `dist/rules.js`
(`webpack.rules.js`) — the latter must stay a single, fully self-contained
ES module (no code splitting, no externals, no dynamic `import()`), since
session-host's V8 isolates have zero runtime module resolution.

`jc deploy` builds a title's `dist/rules.js` and uploads the compiled
bundle to `developer-api`'s provisioning path (`UploadBundle`, into
MinIO) — this packaging contract (`cmd/deploy.go`'s `requiredBundleFiles`/
`createBundleArchive`) didn't change in the 2026-08-05 rewrite, only what
produces those files did. Despite the name, the Game SDK's old
`@jahandco/platform-worker` package was **never** the production execution
path and no longer exists at all (deleted in the same rewrite) — that's
`game-studio`'s `session-host` service, in shared V8 isolates. See
`game-sdk`'s own `CLAUDE.md` for the history there.

**`jc init title`'s scaffold-and-link steps split apart in the same pass
that added `npm/create/`**: `jc init title [name] --local` scaffolds local
files only — no `jc login`, no backend project, nothing written to
`jahandco.config.json` — and `jc project link` (a new subcommand alongside
`project create`/`list`/`delete`) is the separate, explicit step that
resolves-or-creates a backend project, scopes it `title`, and writes
`jahandco.config.json`. `jc init title` with no `--local` still does both
in one step, unchanged, for existing users/scripts. `npm/create/` (see its
own `README.md`) is a small, dependency-free Node package published as
`@jahandco/create` — `npx @jahandco/create@latest my-game` installs `jc`
from the same R2 bucket below if it's missing, then execs `jc init
title my-game --local`. It owns no scaffolding or prompt logic of its own,
purely install-then-delegate — see its own doc comment for why.

## Repo layout

```
main.go              entrypoint, calls cmd.Execute()
cmd/                  one file per top-level command (root, login, project, init, dev, deploy, assets, console)
internal/
  api/                 developer-api HTTP client
  auth/                 Clerk OAuth device/token flow
  templates/             embedded scaffolding templates for `jc init`
npm/create/           @jahandco/create -- npx installer, published to npm separately (see its own README.md)
```

The `jc` CLI itself is still a plain Go module (`go.mod`: `jahandco/cli`),
built/tested directly with the `go` toolchain, no `package.json` at the
repo root. `npm/create/` is the one exception — a small standalone npm
package living alongside it, published independently (manual `npm
publish`, not wired into GoReleaser or CI).

## Build system

```bash
go build ./...                      # build
go test ./...                       # test
go vet ./...                        # vet / "typecheck"
go run . <command>                  # run without building a binary, e.g. go run . login
go install .                        # install `jc` to $GOPATH/bin from source
```

## Release process

See `howto.md` for the full runbook. Short version: tag `v*` and push —
GitHub Actions release workflow (`.github/workflows/release.yml`) runs
GoReleaser (`.goreleaser.yaml`), cross-compiles for linux/darwin/windows
(amd64/arm64), and uploads archives + `checksums.txt` to the `cli-release`
bucket on Cloudflare R2 (moved off self-hosted MinIO). No GitHub/Gitea
Release is created — distribution is that bucket only, downloaded via the
`cli.jahandco.dev` custom domain mapped to it (see the "Installing" section
of `howto.md` for the download URLs).

## CI

GitHub Actions workflows:
- `.github/workflows/test.yml`: runs `go build`/`go test`/`go vet` on every push and pull request.
- `.github/workflows/release.yml`: runs GoReleaser only on `v*` tags (not on branch pushes).
