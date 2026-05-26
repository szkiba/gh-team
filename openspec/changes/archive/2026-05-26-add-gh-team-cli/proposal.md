# Proposal: Add `gh team` CLI extension

## Why
Engineering teams have no consistent, scriptable way to enumerate the repositories they own across a GitHub organization. Today this requires bespoke API scripts or clicking through dozens of browser tabs, making routine maintenance — listing what the team owns, cloning the lot for local work — slow and manual.

## What Changes
- Introduce a new `gh team` extension for the GitHub CLI.
- Add a **Team Ownership Model** resolver that maps `<org>/<team-slug>` to a set of repositories, with two selectable strategies:
  - `permission` (default): the team or any sub-team has the `Admin` or `Maintain` permission on the repository (matching GitHub's `permission` field).
  - `codeowners`: the team owns the `*` (wildcard) pattern in the repository's effective `CODEOWNERS` file on the default branch, respecting last-matching-pattern precedence. Resolution is a two-step pipeline: GitHub code search locates candidates, then each candidate's CODEOWNERS file is fetched from the first existing of `.github/CODEOWNERS`, `CODEOWNERS`, `docs/CODEOWNERS` and parsed exactly.
- Add `team repo list` and `team repo clone` subcommands.
- Add global flags `--ownership`, `--direct-only`, `--include-archived`.
- Reject the combination `--direct-only --ownership=codeowners` with a clear error.
- Run a team-existence preflight (`GET /orgs/<org>/teams/<slug>`) before any resolution, so both ownership strategies behave consistently for stale or mistyped team slugs (rather than `codeowners` silently returning text matches for a team that no longer exists).
- Be explicit in user-facing output that `--ownership=codeowners` makes an exact per-candidate decision but is bounded by GitHub's code search index, so recently added or renamed `CODEOWNERS` files may be missing from the result until they are re-indexed.

## Out of Scope (MVP)
- Security / vulnerability subcommands (`security summary`, `security alerts`). The brief lists them as part of the MVP, but they are deferred to a follow-up change to keep this initial release small and focused on repository discovery. They will be added via a separate change proposal once `team repo` is shipped.

## Impact
- Affected specs: `team-cli` (new), `team-ownership` (new), `team-repo` (new).
- Affected code: new repository — initial implementation.
- External: relies on GitHub REST/GraphQL APIs, GitHub code search API, and `gh repo clone` from the host CLI.
