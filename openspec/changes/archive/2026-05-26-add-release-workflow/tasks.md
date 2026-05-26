# Tasks

## 1. CI workflow
- [x] 1.1 Add `.github/workflows/ci.yml` that triggers on `pull_request` and `push` to `main`
- [x] 1.2 Run `go vet ./cmd/... ./internal/... .` and `go test ./internal/ownership/` in the CI job
- [x] 1.3 Pin Go via `actions/setup-go@v5` with `go-version: '1.25'` to match `go.mod`

## 2. Release workflow
- [x] 2.1 Add `.github/workflows/release.yml` triggered on tag `v*`
- [x] 2.2 Wire `cli/gh-extension-precompile@v2` so the canonical matrix of platform binaries is produced
- [x] 2.3 Grant `contents: write` permission to the job so it can create the GitHub Release
- [x] 2.4 Run the test suite as a release-prerequisite step so a failing test blocks the publish

## 3. Docs
- [x] 3.1 README: add a "Releasing" section that explains the tag flow (`git tag vX.Y.Z && git push --tags`)
- [x] 3.2 README: update the "Installation" section to reference `gh extension install szkiba/gh-team` once a release exists, and remove the "not published yet" caveat
