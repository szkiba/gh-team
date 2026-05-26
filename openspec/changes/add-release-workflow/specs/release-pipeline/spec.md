# release-pipeline Specification

## ADDED Requirements

### Requirement: Tagged releases publish platform binaries
The project SHALL publish a GitHub Release whenever a tag matching `v*` is pushed to the repository. The release SHALL include precompiled `gh-team` binaries for the canonical `gh` extension target matrix (linux amd64/arm64, darwin amd64/arm64, windows amd64/arm64), named so `gh extension install <owner>/gh-team` can select the correct binary for the host platform.

#### Scenario: Pushing a semver tag triggers a release
- **GIVEN** the repository has a green `main` branch
- **WHEN** a maintainer pushes a tag `v1.2.3`
- **THEN** GitHub Actions runs the release workflow
- **AND** a GitHub Release named `v1.2.3` is created with attached precompiled binaries
- **AND** the binaries are named in the form `gh-team_<version>_<os>_<arch>[.exe]` so `gh extension install` resolves the right asset

### Requirement: Release blocks on failing tests
The release workflow SHALL run the project's test suite before producing artifacts. If `go test` fails, the workflow SHALL fail and no GitHub Release SHALL be created, so a tag cannot ship a binary that the test suite would have flagged.

#### Scenario: Failing tests on a tagged commit block the release
- **GIVEN** the test suite is broken on a commit
- **WHEN** that commit is tagged and the tag is pushed
- **THEN** the release workflow fails at the test step
- **AND** no GitHub Release is created
- **AND** no binaries are published

### Requirement: Pull requests run tests and vet
The project SHALL run `go test` and `go vet` on every pull request and on every push to the `main` branch, so the test suite is enforced before merge — not only at release time.

#### Scenario: PR with failing tests is flagged
- **GIVEN** a pull request whose changes break a test
- **WHEN** the PR is opened or updated
- **THEN** the CI workflow runs and fails
- **AND** the failing check is visible on the PR before review

### Requirement: Documented release flow
The project README SHALL document the release procedure (tag a commit, push the tag, wait for the release to appear), so a maintainer who has not run a release before can ship a version without reading the workflow source.

#### Scenario: A new maintainer can release without reading workflow source
- **GIVEN** a maintainer has merge rights but has never run a release for this project
- **WHEN** they consult the README's "Releasing" section
- **THEN** they find the exact `git tag vX.Y.Z && git push --tags` invocation
- **AND** they find the URL pattern under which the resulting release will appear
- **AND** they can ship a release without opening any file under `.github/workflows/`
