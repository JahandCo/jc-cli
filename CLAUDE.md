# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

**`jc`** is the developer CLI for the Jah and Co Developer Platform
(`jahandco.dev`) — `jc login`, `jc project`, `jc init title`, `jc dev`,
`jc deploy`, `jc assets`, `jc console`. It split out of `developer-platform`
on 2026-08-04, where it lived as `services/cli` — a plain Go module with no
`package.json`, not wired into that repo's pnpm/Turborepo pipeline. See
`developer-platform`'s own `CLAUDE.md` for the history.

`jc init` scaffolds a new title against the [Game SDK](https://github.com/JahandCo/game-sdk)
(`@jahandco/interactive-sdk` etc. — a separate repo, consumed as published
npm dependencies, never a workspace link). `jc deploy` builds a title's
`dist/rules.js` and uploads the compiled bundle to `developer-api`'s
provisioning path (`UploadBundle`, into MinIO). Despite the name, the
Game SDK's `@jahandco/platform-worker` package is **not** what runs it in
production — that's `game-studio`'s `session-host` service, in shared V8
isolates. See `game-sdk`'s own `CLAUDE.md`/`runtime/README.md` for the
history there.

## Repo layout

```
main.go              entrypoint, calls cmd.Execute()
cmd/                  one file per top-level command (root, login, project, init, dev, deploy, assets, console)
internal/
  api/                 developer-api HTTP client
  auth/                 Clerk OAuth device/token flow
  templates/             embedded scaffolding templates for `jc init`
```

No `package.json` — this is a plain Go module (`go.mod`: `jahandco/cli`),
built/tested directly with the `go` toolchain.

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
(amd64/arm64), and uploads archives + `checksums.txt` to the self-hosted
MinIO `cli-releases` bucket. No GitHub/Gitea Release is created —
distribution is that bucket only (see the "Installing" section of `howto.md`
for the download URLs).

## CI

GitHub Actions workflows:
- `.github/workflows/test.yml`: runs `go build`/`go test`/`go vet` on every push and pull request.
- `.github/workflows/release.yml`: runs GoReleaser only on `v*` tags (not on branch pushes).
