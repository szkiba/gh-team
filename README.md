# gh team

`gh team` is a GitHub CLI (`gh`) extension for discovering repositories owned by a GitHub team and reporting on open security alerts across them.

It is designed for teams that need a consistent, scriptable way to answer questions like "which repositories do we own?" and "where are our open Dependabot or code-scanning alerts?" without writing custom API scripts or clicking through the GitHub UI.

## Status

Repository discovery (`team repo list`, `team repo clone`) and read-only security inspection (`team security summary`, `team security alerts`) are implemented and tested. The security commands cover open Dependabot and code-scanning alerts; secret scanning is intentionally out of scope for this release.

## Install

Requires the `gh` CLI on `PATH` and an authenticated session (`gh auth login`).

The recommended path is to install the published extension:

```bash
gh extension install szkiba/gh-team
gh team --help
```

`gh` will download the precompiled binary for your platform from the most recent GitHub Release. If no release matches your platform, or you want the latest unreleased code, build from a checkout:

```bash
git clone https://github.com/szkiba/gh-team.git
cd gh-team
go build -o gh-team .
./gh-team --help
```

Or install the checkout as a local extension:

```bash
gh extension install .
gh team --help
```

Building from source requires Go 1.25+.

## Commands

```text
gh team repo list <org/team-slug>
gh team repo clone <org/team-slug>
gh team security summary <org/team-slug> [--kind=dependabot|code-scanning|all]
gh team security alerts  <org/team-slug> [--kind=dependabot|code-scanning|all]
```

### Global flags

- `--ownership=permission|codeowners` (default `permission`) — selects the ownership strategy.
- `--direct-only` — evaluates only repositories assigned directly to the top-level team, skipping sub-teams. Only valid with `--ownership=permission`; `--direct-only --ownership=codeowners` is rejected with an error because CODEOWNERS has no team hierarchy to limit.
- `--include-archived` — includes archived repositories in the result (excluded by default).

### Output flags

Data-emitting subcommands (`repo list`, `security summary`, `security alerts`) accept three optional output flags. `--json` and `--template` are mutually exclusive output modes; `--header` is a modifier on default (TSV) mode and is rejected when combined with either output mode. Default behavior is unchanged when no flag is set.

- `--json` — emits a single JSON array, one object per item, sorted in the same order as default mode. A trailing newline is appended for shell friendliness. Empty result sets emit `[]\n`.
- `--template <go-template>` — runs the supplied Go `text/template` once per item and emits exactly one line per execution. Items are rendered in the same order as default mode. The template engine is configured with `missingkey=error` so a typo against an unknown field (`{{.full_nam}}`) is reported as an execution error rather than rendering `<no value>`. Templates that produce more than one line per item are rejected with an explicit error.
- `--header` — prepends a single tab-separated header line of field names in default TSV mode for direct import into Excel or Google Sheets. The header line is emitted even when the result is empty. For `gh team repo list`, setting `--header` also widens each data row to the same four-column TSV named in the header (`owner\tname\tfull_name\tarchived`) — the no-flag default stays single-column. `security summary` and `security alerts` default rows already match their header columns.

`gh team repo clone` does not accept these flags — it is a side-effect command without a dataset stdout contract.

#### Field names

The field names below are part of the public output contract. Additive fields may be introduced in future versions; existing names will not be removed or renamed without a separate compatibility decision.

| Command | Fields |
| --- | --- |
| `repo list` | `.owner`, `.name`, `.full_name`, `.archived` |
| `security summary` | `.repo`, `.family`, `.count` |
| `security alerts` | `.family`, `.repo`, `.key`, `.severity`, `.url` |

#### Deferred output ideas

- `--output <path>`: deferred — shell redirection covers it; file-overwrite semantics and permission handling can come later.
- `--format tsv|json|template`: deferred — explicit flags are clearer for v1.
- `--markdown`: deferred — a markdown table output mode is scoped for a follow-up change.
- `--no-warnings`: rejected for now — would hide partial-failure information important for security commands.
- `--color` / table rendering: deferred — the project favors deterministic pipe-friendly output over terminal decoration.

## Ownership models

`gh team` supports two ownership strategies selected with `--ownership`:

- `permission` (default): a repository is owned when the team, or any of its sub-teams, has `Admin` or `Maintain` permission on it.
- `codeowners`: a repository is owned when the team appears on the last bare `*` rule in the effective `CODEOWNERS` file on the default branch. The effective file is the first existing file in this order: `.github/CODEOWNERS`, `CODEOWNERS`, `docs/CODEOWNERS`. Path-scoped entries are ignored in the MVP; only the bare `*` rule is consulted.

### `codeowners` caveat

The `codeowners` strategy uses GitHub code search to find candidate repositories, then fetches and parses the effective `CODEOWNERS` file exactly. That makes the per-repository decision exact for each candidate, but the overall result can lag if GitHub has not re-indexed a recently added, renamed, or moved `CODEOWNERS` file yet.

It also costs more API work than `permission`: one code-search request plus one `contents` fetch per candidate repository. On large organizations, code-search rate limits can become the main constraint.

Whenever `--ownership=codeowners` is used, the command prints a one-line note to `stderr` stating that the result is based on GitHub's code search index and may omit recently added or renamed `CODEOWNERS` files until they are re-indexed. The note does not change `stdout` or the exit status.

## Usage

List repositories owned by a team using the default `permission` strategy:

```bash
gh team repo list octo/platform
```

Example output:

```text
octo/api
octo/ingestor
octo/web
```

List repositories using the `codeowners` strategy:

```bash
gh team repo list octo/platform --ownership=codeowners
```

List only repositories directly assigned to the top-level team:

```bash
gh team repo list octo/platform --direct-only
```

Include archived repositories:

```bash
gh team repo list octo/platform --include-archived
```

Clone all owned repositories into the current directory:

```bash
gh team repo clone octo/platform
```

Pipe results into another command:

```bash
gh team repo list octo/platform | xargs -L1 gh repo view
```

JSON output for scripting:

```bash
gh team repo list octo/platform --json | jq '.[] | select(.archived | not) | .full_name'
```

Custom one-line-per-repo rendering:

```bash
gh team repo list octo/platform --template '{{.full_name}} archived={{.archived}}'
```

Labeled TSV with header line for spreadsheet import:

```bash
gh team repo list octo/platform --header --include-archived
```

Example output:

```text
owner	name	full_name	archived
octo	api	octo/api	false
octo	legacy	octo/legacy	true
```

### Security alerts

`gh team security summary` prints open alert counts per owned repository and family. Output is tab-separated, sorted by repository then family, and lines with zero open alerts are dropped:

```bash
gh team security summary octo/platform
```

Example output:

```text
octo/api	code-scanning	1
octo/api	dependabot	2
octo/web	code-scanning	3
```

`gh team security alerts` prints one tab-separated line per open alert. Columns are `family`, `<org>/<repo>`, key, severity, URL. The Dependabot key is `<ecosystem>:<package>@<manifest-path>`; the code-scanning key is the rule id. Code-scanning severity prefers `security_severity_level` and falls back to `rule.severity`:

```bash
gh team security alerts octo/platform
```

Restrict to a single family with `--kind`:

```bash
gh team security alerts octo/platform --kind=dependabot
gh team security summary octo/platform --kind=code-scanning
```

JSON output for scripting:

```bash
gh team security summary octo/platform --json | jq '.[] | select(.count > 5)'
gh team security alerts  octo/platform --json | jq '.[] | select(.severity == "high")'
```

Custom one-line-per-item rendering:

```bash
gh team security summary octo/platform --template '{{.repo}} {{.family}}={{.count}}'
gh team security alerts  octo/platform --template '{{.severity}} {{.repo}} {{.url}}'
```

Labeled TSV with header line for spreadsheet import:

```bash
gh team security summary octo/platform --header
gh team security alerts  octo/platform --header
```

`--kind=all` is a fixed alias for the union of `dependabot` and `code-scanning`. Secret scanning is intentionally excluded; a future family must be requested by name until a separate compatibility decision updates the alias.

#### Maintainer baseline

The security commands assume the caller has at least repository `maintain` permission on each owned repository. That baseline maps cleanly to `--ownership=permission`.

With `--ownership=codeowners` the resolver can surface repositories the caller cannot read alerts for — wildcard CODEOWNERS ownership does not imply repository access. In that case the command continues, prints a per-repository warning to `stderr` naming the affected repository and alert family, and exits non-zero after rendering any successful results.

Repositories where an alert family is simply not enabled (for example code scanning not configured) contribute no output line and no warning. A missing OAuth scope for security alerts is surfaced once with guidance to run `gh auth refresh -s read:org,security_events`.

The security subcommands provide first-class auth guidance only for classic `gh auth` OAuth sessions, which is what `gh auth login` produces. Fine-grained personal access tokens and GitHub App tokens use dedicated repository permissions (`Dependabot alerts: read`, `Code scanning alerts: read`) instead of OAuth scopes; with one of those token types, a missing permission falls through the OAuth-scope detection and surfaces as a per-repository access-denied warning. The remedy in that case is to widen the token's repository permissions, not to run `gh auth refresh`.

## Behavior

- Team arguments use the form `<org>/<team-slug>`.
- Repository output is printed one full repository name per line in `<org>/<repo>` form, sorted alphabetically. With `gh team repo list --header`, each row widens to the four-column TSV `owner\tname\tfull_name\tarchived` documented in the output flags section.
- Archived repositories are excluded unless `--include-archived` is set.
- `gh team repo clone` delegates cloning to `gh repo clone` and clones into subdirectories of the current working directory.
- If a destination directory already exists, the clone for that repository is skipped, a non-fatal warning is printed to `stderr`, and the remaining clones still run.
- Clone operations continue past per-repository failures and exit non-zero if any clone failed.
- Missing authentication is surfaced with guidance to run `gh auth login`.
- Missing scopes are surfaced with actionable guidance such as `gh auth refresh -s read:org`; for private repositories, `codeowners` may additionally require `gh auth refresh -s read:org,repo`, and `security` subcommands surface `gh auth refresh -s read:org,security_events` when the OAuth session lacks the alert-read scope.
- Security subcommands continue past per-repository alert-access failures, print warnings to `stderr` naming each affected repository and family, and exit non-zero if any hard failure occurred.

### Exit behavior

- Success returns exit status `0`, including when no repositories match.
- Invalid team arguments, missing teams, invalid flag combinations, authentication failures, and rate-limit failures return a non-zero exit status.
- Rate-limit errors name the affected limit (core REST, GraphQL, or code search) and an absolute UTC reset time. The reset time comes from `X-RateLimit-Reset` for primary limits and from `Retry-After` (delta-seconds or HTTP-date) for secondary / abuse-detection limits. When GitHub returns a secondary-limit response without either header, the message says so explicitly and recommends waiting a few minutes before retrying.

## Releasing

A release is cut by pushing a semver tag. The [`release`](./.github/workflows/release.yml) workflow runs the test suite, cross-compiles `gh-team` for the canonical `gh` extension target matrix (linux/darwin/windows × amd64/arm64), and attaches the binaries to a GitHub Release named after the tag.

```bash
git tag v0.1.0
git push --tags
```

The release will appear at `https://github.com/szkiba/gh-team/releases/tag/v0.1.0` once the workflow finishes. From that point on, `gh extension install szkiba/gh-team` downloads the precompiled binary for the user's platform. A failing test blocks the release; no binaries are published.

## Repository layout

- [`brief.md`](./brief.md) — product brief for the MVP.
- [`openspec/project.md`](./openspec/project.md) — project context, constraints, and conventions.
- [`openspec/specs/`](./openspec/specs/) — current, archived specs (`team-cli`, `team-ownership`, `team-repo`).
- [`openspec/changes/`](./openspec/changes/) — in-flight and archived change proposals.
- [`cmd/`](./cmd/) — Cobra command tree.
- [`internal/ownership/`](./internal/ownership/) — strategy-agnostic resolver, both ownership strategies, and CODEOWNERS parser.
- [`internal/security/`](./internal/security/) — repository-level Dependabot and code-scanning alert collector with bounded concurrency.
- [`.github/workflows/`](./.github/workflows/) — CI and release pipelines.
