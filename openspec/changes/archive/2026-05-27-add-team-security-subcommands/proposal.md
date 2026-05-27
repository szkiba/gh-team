## Why

The repository-discovery MVP is already in place, but maintainers still have to click through individual repository security pages to understand the open Dependabot and code-scanning work for the repositories their team owns. The organization-wide security APIs are optimized for organization owners and security managers, which makes them a poor fit for the baseline `gh team` persona of a repository maintainer.

## What Changes

- Add a new `gh team security` command area under the existing `gh team` root.
- Add `gh team security summary <org/team-slug>` to show open alert counts per owned repository and alert family.
- Add `gh team security alerts <org/team-slug>` to list individual open alerts across owned repositories.
- Resolve repositories through the existing shared ownership resolver and inherited global flags (`--ownership`, `--direct-only`, `--include-archived`) before collecting security data.
- Collect every page of alert results for each repository and requested alert family before rendering summary counts or alert lines.
- Use repository-level GitHub REST endpoints so the baseline persona can be a repository maintainer rather than an organization owner or security manager.
- Support `dependabot` and `code-scanning` in the MVP via `--kind=dependabot|code-scanning|all` (`all` default), with `all` frozen to those two families for compatibility in this change line.
- Exclude `secret-scanning` from the MVP because its repository list APIs require stricter permissions than the baseline maintainer persona can rely on.
- Use bounded repository-level fanout so multi-repository security queries do not become purely sequential on large teams.
- Keep the MVP read-only: no alert dismissal, reopening, autofix, or bulk mutation commands.
- Document that the maintainer baseline maps cleanly to `--ownership=permission`; when `--ownership=codeowners` returns repositories the caller cannot read alerts for, the command warns per repository and exits non-zero after processing the full set.

## Capabilities

### New Capabilities
- `team-security`: Read-only security visibility for the repositories owned by a team, scoped to Dependabot and code-scanning alerts.

### Modified Capabilities
- `team-cli`: Extend authentication and scope guidance so security subcommands surface actionable fixes for missing security-alert scopes.

## Impact

- Affected code: Cobra command tree under `cmd/`, new security collection/output package(s) under `internal/`, README/help text, and tests.
- External APIs: GitHub REST repository endpoints for Dependabot alerts and code-scanning alerts.
- User-facing behavior: new `gh team security` subcommands, new `--kind` flag, paginated alert collection, and new partial-failure warnings for per-repository security access problems.
