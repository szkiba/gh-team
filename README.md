# gh team

`gh team` is a GitHub CLI (`gh`) extension for discovering repositories owned by a GitHub team.

It is designed for teams that need a consistent, scriptable way to answer questions like "which repositories do we own?" without writing custom API scripts or clicking through the GitHub UI.

## Status

The MVP (`team repo list`, `team repo clone`) is implemented and tested. The MVP is intentionally focused on repository discovery and cloning. Security and vulnerability commands are tracked for a follow-up change once the shared ownership resolver is in production use.

## Install

Requires the `gh` CLI on `PATH` and an authenticated session (`gh auth login`).

From source:

```bash
go build -o gh-team .
./gh-team --help
```

Or as a local `gh` extension from a checkout:

```bash
gh extension install .
gh team --help
```

Building from source requires Go 1.25+. The extension is not yet published to a registry; once it is, the usual flow will work:

```bash
gh extension install szkiba/gh-team   # not published yet
```

## Commands

```text
gh team repo list <org/team-slug>
gh team repo clone <org/team-slug>
```

### Global flags

- `--ownership=permission|codeowners` (default `permission`) — selects the ownership strategy.
- `--direct-only` — evaluates only repositories assigned directly to the top-level team, skipping sub-teams. Only valid with `--ownership=permission`; `--direct-only --ownership=codeowners` is rejected with an error because CODEOWNERS has no team hierarchy to limit.
- `--include-archived` — includes archived repositories in the result (excluded by default).

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

## Behavior

- Team arguments use the form `<org>/<team-slug>`.
- Repository output is printed one full repository name per line in `<org>/<repo>` form, sorted alphabetically.
- Archived repositories are excluded unless `--include-archived` is set.
- `gh team repo clone` delegates cloning to `gh repo clone` and clones into subdirectories of the current working directory.
- If a destination directory already exists, the clone for that repository is skipped, a non-fatal warning is printed to `stderr`, and the remaining clones still run.
- Clone operations continue past per-repository failures and exit non-zero if any clone failed.
- Missing authentication is surfaced with guidance to run `gh auth login`.
- Missing scopes are surfaced with actionable guidance such as `gh auth refresh -s read:org`; for private repositories, `codeowners` may additionally require `gh auth refresh -s read:org,repo`.

### Exit behavior

- Success returns exit status `0`, including when no repositories match.
- Invalid team arguments, missing teams, invalid flag combinations, authentication failures, and rate-limit failures return a non-zero exit status.
- Rate-limit errors name the affected limit (core REST, GraphQL, or code search) and the absolute UTC reset time taken from the response headers.

## Repository layout

- [`brief.md`](./brief.md) — product brief for the MVP.
- [`openspec/project.md`](./openspec/project.md) — project context, constraints, and conventions.
- [`openspec/changes/add-gh-team-cli/`](./openspec/changes/add-gh-team-cli/) — proposal, tasks, and detailed specs.
- [`cmd/`](./cmd/) — Cobra command tree.
- [`internal/ownership/`](./internal/ownership/) — strategy-agnostic resolver, both ownership strategies, and CODEOWNERS parser.
