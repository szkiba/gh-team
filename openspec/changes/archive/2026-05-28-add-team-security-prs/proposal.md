## Why

`gh team security summary` and `gh team security alerts` tell the maintainer what is broken, but not what is being fixed. To close the loop, a team lead currently has to run `gh search prs` or `gh pr list` per repository and post-filter by title or label. That falls outside the team-ownership model and produces inconsistent results. A team-scoped command that surfaces open PRs matching security signals completes the read-only security triad and keeps repository selection identical across `repo` and `security`.

## What Changes

- Add `gh team security prs <team>` subcommand. Lists open pull requests in the team-owned repository set whose title matches a security regex OR whose label set contains a security label.
- Default title regex: `(?i)^\[security\]|^security:|\[security\]$`. Default label: `security` (exact). Signals are OR-combined; the combiner is not user-tunable in v1.
- Overrides: `--title <regex>` replaces the title default. `--label <l>` replaces the label default and is repeatable.
- Reuse the v0.4.0 output-flags contract: `--header`, `--json`, `--template` work identically; `--header` is rejected with `--json` or `--template`, and `--json` and `--template` remain mutually exclusive.
- Default-mode row shape: `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl` (seven tab-separated columns; `--header` line `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl`).
- Sort: `repo` ascending, then `number` descending (newest PR first within a repo).
- PR titles containing `\t` or `\n` are sanitized to a single space in default and `--header` mode. `--json` preserves the original title verbatim.
- Per-repository fetch failures follow the existing `security alerts` partial-failure pattern: stderr warning naming the repository, command continues for other repos, exit status `1` after rendering.

## Capabilities

### New Capabilities

(none — this slots into the existing security area)

### Modified Capabilities

- `team-security`: add a new requirement for the `prs` subcommand, its default signals, override flags, sort, row shape, title sanitization rule, and partial-failure behavior.
- `team-cli`: add a requirement that `--header` is rejected with `--json` or `--template` on the new `security prs` subcommand (same rule already documented for the other data-emitting subcommands).

## Impact

- New file `cmd/security_prs.go` plus tests, registered under the existing `security` cobra group in `cmd/security.go`.
- New PR-collection plumbing in `internal/security/` (or a sibling package) reusing the concurrency / partial-failure model of the alert collector.
- README updated: `gh team security prs` examples, field-name contract row added, default regex documented.
- Auth: listing pull requests on **private** owned repositories requires repository-read access on the host `gh` session — that is, the classic OAuth `repo` scope (or an equivalent fine-grained `Pull requests: read` permission), in addition to the `read:org` scope already needed for ownership resolution. Public repositories list without a repository-read scope. The shared `team-cli` auth-guidance requirement is extended to cover this command family so a session that can resolve ownership but cannot read pull requests gets remediation that names the missing scope rather than a generic 403.
- Non-goals (v1): `--state` flag (open only), author-based defaults, `--any`/`--all` combiner, alert↔PR cross-linking, mutating PR actions, and the `secret-scanning` family — each tracked as a possible follow-up.
