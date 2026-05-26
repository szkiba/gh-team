# Project Context

## Purpose
`gh team` is a GitHub CLI (`gh`) extension that discovers repositories owned by a GitHub team. Ownership is resolved through a pluggable **Team Ownership Model** with two strategies:

- **`permission`** (default): a repository belongs to the team if that team — or any sub-team — holds the `Admin` or `Maintain` permission on it (matching GitHub's `permission` field on team-to-repository bindings).
- **`codeowners`**: a repository belongs to the team if the team is named as the owner of the `*` (wildcard) pattern in the repository's `CODEOWNERS` file on the default branch.

The extension exists because engineering teams currently lack a single consistent way to enumerate the repositories they own (directly or transitively). Today this requires custom API scripts or manual browser navigation. A follow-up change will add security/vulnerability subcommands on top of the same ownership resolver.

## Tech Stack
- Language: Go (standard for `gh` extensions)
- Distribution: `gh extension install` (GitHub CLI extension model)
- APIs: GitHub REST API and/or GraphQL API, accessed via the authenticated `gh` session
- Cloning: delegated to `gh repo clone` so auth, protocol choice (SSH/HTTPS), and credential handling stay consistent with the host CLI

## Project Conventions

### Code Style
- Idiomatic Go; `gofmt`/`goimports` clean.
- Cobra-style subcommands consistent with other `gh` extensions.

### Architecture Patterns
- Subcommand tree: `gh team repo <action> <org/team-slug>` for the MVP. Additional areas (e.g. `security`) will be added under the same root in later changes.
- A single shared ownership resolver is used by every subcommand so future areas reuse identical ownership semantics.
- The resolver selects a strategy at runtime based on `--ownership`.
- Global flags (`--ownership`, `--direct-only`, `--include-archived`) are parsed once at the root and threaded into the resolver.
- External commands (`gh repo clone`) are invoked via the host `gh` binary rather than reimplementing git plumbing.

### Testing Strategy
- Unit tests for each ownership strategy (permission strategy: sub-team recursion + permission-level filtering; CODEOWNERS wildcard parsing).
- Integration tests stubbed against recorded GitHub API responses.

### Git Workflow
- Trunk-based; feature branches merged via PR.

## Domain Context
- **Owning team (permission strategy)**: a GitHub team with `Admin` or `Maintain` permission on a repository, directly or transitively through sub-teams.
- **Owning team (codeowners strategy)**: a GitHub team named as the owner of the `*` pattern in the repository's `CODEOWNERS` file on the default branch. Path-scoped entries are ignored in the MVP.
- **CODEOWNERS resolution**: candidate repositories are located via GitHub code search with the broad query `org:<ORG> path:CODEOWNERS "@<ORG>/<TEAM>"`. For each candidate the effective CODEOWNERS file is then fetched on the default branch from the first existing of `.github/CODEOWNERS`, `CODEOWNERS`, `docs/CODEOWNERS` (GitHub's resolution order; later locations are ignored once an earlier one is found). The file is parsed line-by-line:
  - Comments are stripped: on each line, everything from the first `#` character to end-of-line is discarded.
  - Each remaining non-blank line is tokenized on whitespace; the first token is the pattern, the rest are owners.
  - Ownership of the repository by the team is established when the **last** line whose first token is exactly `*` (the bare wildcard) lists `@<ORG>/<TEAM>` as one of its owners. This respects CODEOWNERS' last-matching-pattern precedence.
- The code search step is a best-effort *candidate filter* only; the per-candidate ownership decision is exact, but the candidate set itself is limited by what GitHub's code search index currently contains. Recently added, renamed, or moved `CODEOWNERS` files may be missing from the index temporarily, so the returned result set can lag real repository state. The spec therefore claims exact semantics at the *per-repository* level, not at the *organization-wide completeness* level. Implementations and docs SHALL communicate this limitation to users; users who need org-wide completeness can rerun the command after indexing catches up.
- **Direct-only**: ownership evaluated for the top-level team only, sub-teams excluded. Only meaningful in the `permission` strategy.
- **Active repository**: not archived. Default scope; archived repos opt-in via `--include-archived`.

## Important Constraints
- Authentication and rate limits are inherited from the host `gh` session; no separate token management.
- Org/team slug must be passed as `<org>/<team-slug>` on every command.
- `--direct-only` combined with `--ownership=codeowners` is rejected as an error; CODEOWNERS has no team hierarchy.
- The `codeowners` strategy uses GitHub code search to locate candidate repositories and then issues one `contents` REST call per candidate to fetch and parse the file. Code search has a separate (lower) rate limit than the REST API and only indexes default branches; recently added or renamed CODEOWNERS files may be missing until GitHub re-indexes them.
- MVP is limited to listing and cloning owned repositories — no mutation of team membership or repository settings, and no security/vulnerability features in this release.

## External Dependencies
- GitHub CLI (`gh`) — host process, auth provider, and cloning backend (via `gh repo clone`).
- GitHub REST/GraphQL APIs — source of teams and repositories.
- GitHub code search API — used to locate CODEOWNERS files mentioning the team.
