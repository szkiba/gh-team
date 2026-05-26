# Proposal: Add release workflow for `gh team`

## Why
The MVP is shipped and verified, but the extension is only installable by users who have a Go toolchain and clone the repo. Real users will run `gh extension install szkiba/gh-team` and expect a precompiled binary for their platform. Without a release workflow, every install falls back to source-build, which fails for users without Go and silently breaks Windows users entirely (different defaults, no consistent toolchain).

CI coverage is also unrun on PRs today; the tests pass locally but nothing enforces them at PR time.

## What Changes
- Add a **release workflow** (`.github/workflows/release.yml`) triggered by `vX.Y.Z` tags. It uses [`cli/gh-extension-precompile`](https://github.com/cli/gh-extension-precompile), the canonical action for `gh` extensions, to cross-compile and attach platform binaries to a GitHub Release named after the tag.
- Cross-compile target matrix matches what the precompile action ships by default for Go extensions: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64, windows/arm64.
- Add a **CI workflow** (`.github/workflows/ci.yml`) that runs `go test ./...` and `go vet ./...` on every pull request and every push to `main`, so the test suite gates the release tag rather than being advisory.
- Document the release flow (tag a commit, push, wait for the release) in `README.md` under a new "Releasing" section.

## Out of Scope
- Changelog generation. Manual release notes are fine for the MVP volume; auto-generation can come as a follow-up if release frequency justifies it.
- Signed releases / SLSA provenance. Useful eventually, not part of MVP.
- A separate "nightly" or pre-release channel.

## Impact
- Affected specs: `release-pipeline` (new).
- Affected code: only `.github/workflows/*.yml` and a short README addition.
- External: GitHub Actions minutes (free on public repos), one new third-party action (`cli/gh-extension-precompile`, owned by the `gh` CLI org).
