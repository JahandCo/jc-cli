# @jahandco/create

`npx @jahandco/create@latest my-game` — the zero-install entry point for
trying the Jah and Co Game SDK. No account, no prior `jc` install required.

## What it does

1. Checks whether `jc` (this repo's own CLI) is already on `PATH`.
2. If not, resolves the current release version from
   `packages.jahandco.dev/jc-cli/latest.txt` (unless `JC_VERSION` is set to
   pin one), downloads the matching archive for your OS/arch from the same
   `jc-cli/` prefix (a custom domain mapped to the `releases` Cloudflare R2
   bucket `jc-cli`'s GoReleaser release process uploads to — see
   `../.goreleaser.yaml`), and installs it to `~/.jahandco/bin`.
3. Runs `jc init title <args> --local` — every actual scaffolding step and
   interactive prompt comes from `jc` itself (see `../../cmd/init.go`).
   This script has no scaffolding logic or prompts of its own.

Once you're ready to connect the scaffolded directory to a real backend
project: `jc login`, then `jc project link` (see `../../cmd/project.go`).

## Releasing a new version

Nothing to do here — `latest.txt` is written fresh to
`packages.jahandco.dev/jc-cli/latest.txt` by `jc-cli`'s own GoReleaser
config (`.goreleaser.yaml`'s `before.hooks`) on every tagged release, and
this script reads it at runtime. This package's own version
(`package.json`) only needs a bump/`npm publish` when `create.js` itself
changes, not when `jc-cli` cuts a new CLI release:

```bash
npm publish
```

from this directory. Same manual-publish convention `game-sdk`'s own
npm packages already follow (see that repo's `sdk-changelog.md`) — nothing
in `jc-cli`'s CI publishes this package automatically.
