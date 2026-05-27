## Context

`gh team` already has a stable root command, shared ownership resolver, and two shipped repo-oriented subcommands. This change adds a new command area on top of that resolver rather than introducing a second notion of repository scope.

The baseline persona for this change is a user who has `maintain` permission on the repositories returned by the ownership resolver. That persona can work with Dependabot and code-scanning alerts at the repository level, but cannot reliably use organization-level security aggregation endpoints or secret-scanning list endpoints.

That baseline maps cleanly to `--ownership=permission`, because the permission strategy already filters to repositories where the team has `Admin` or `Maintain`. It does not map perfectly to `--ownership=codeowners`: a team can own the wildcard CODEOWNERS line for a repository without having maintain-level repository access. In those cases, the security commands will still traverse the resolved set, warn for inaccessible repositories, and exit non-zero after rendering any successful results.

## Goals / Non-Goals

**Goals:**
- Add `gh team security summary` and `gh team security alerts` as read-only commands.
- Reuse the existing ownership model and root flags so repository selection stays identical across `repo` and `security`.
- Make the commands useful for a repository maintainer without requiring organization-owner or security-manager privileges.
- Keep stdout deterministic and scriptable.
- Continue processing other repositories when one repository cannot be queried, while still surfacing failures clearly.
- Bound request fanout so multi-repository runs are materially faster than a fully sequential collector while still staying within the existing rate-limit/error model.

**Non-Goals:**
- Secret scanning in the MVP.
- Alert mutation flows such as dismiss, reopen, assign, or autofix.
- Organization-wide alert aggregation via org security endpoints.
- Historical reporting across dismissed/fixed/resolved states.
- Adding new user-facing throttling flags such as `--concurrency` or `--max-repos` in the MVP.

## Decisions

### 1. `security` is a sibling of `repo`

The new command surface will be `gh team security <action>`, parallel to `gh team repo <action>`.

Why:
- The current root command already owns the shared flags and ownership semantics.
- Security is a separate user intent from repository listing/cloning.
- This leaves room for multiple security-oriented subcommands without overloading `repo`.

Alternative considered:
- Put security under `gh team repo security ...`.
  Rejected because it nests a second concern under a command area that already means "operate on repositories directly," while security needs its own flags, output formats, and error model.

### 2. Use repository-level alert collection after ownership resolution

The implementation will resolve the repository set first, then query repository-level REST endpoints for each requested alert family.

Why:
- Organization-level alert endpoints require organization owner or security manager permissions, which do not match the baseline maintainer persona.
- The existing resolver already gives the exact repository scope users expect.
- Repo-by-repo collection composes cleanly with both ownership strategies and archived filtering.

Alternatives considered:
- Organization-level alert listing plus local filtering.
  Rejected because the baseline persona cannot rely on those permissions.
- Separate security-specific repository discovery.
  Rejected because it would diverge from the central product promise that every `gh team` area operates on the same owned-repository set.

### 3. MVP covers Dependabot and code scanning only

The MVP will support `--kind=dependabot|code-scanning|all`, with `all` meaning exactly those two families.

Why:
- Maintainers can act on Dependabot and code-scanning alerts at the repository level.
- Secret-scanning repository list APIs require repository or organization administrators, which is stricter than the baseline persona.
- Limiting the MVP keeps the contract honest and reduces confusing permission failures.

Alternative considered:
- Include secret scanning behind the same default surface.
  Rejected because it would make the maintainer baseline false in common cases.

### 4. `all` is a stable compatibility alias in v1

Within this change line, `all` will remain an alias for exactly `dependabot` and `code-scanning`. If a later change adds a new alert family, that family must be explicitly requested by name until a separate compatibility decision revisits the alias.

Why:
- Automation that relies on stable counts should not change behavior just because the binary learned a new family.
- Freezing the alias makes the MVP safer for shell pipelines and CI.

Alternative considered:
- Make `all` expand to every family the binary supports at runtime.
  Rejected because it silently changes command meaning across releases.

### 5. Output is line-oriented and deterministic

`summary` and `alerts` will emit tab-separated, headerless output sorted deterministically.

Why:
- Existing `gh team` commands favor pipe-friendly stdout with no decoration.
- A fixed TSV shape is easier to test and more shell-friendly than an aligned table.
- Deterministic ordering keeps automation stable and mirrors the existing resolver behavior.

Alternative considered:
- Human-oriented tables by default.
  Rejected because column widths and decoration are harder to script against.

### 6. Security collection is best-effort across repositories

Repository-level permission or scope failures will not abort the entire run immediately. The command will continue across the remaining repositories, emit warnings for the repositories/families that could not be queried, and exit non-zero if any hard failures occurred. Repositories where an alert family is simply unavailable or disabled will be treated as yielding zero alerts.

Why:
- Multi-repository fanout should preserve as much useful output as possible.
- Feature-disabled repositories are common and should not feel like command failure.
- Hard permission/scope failures still need a non-zero exit so automation can detect incomplete results.

Alternative considered:
- Fail fast on the first repository error.
  Rejected because one inaccessible repository would hide useful results for the rest of the team’s owned set.

### 7. Collect all pages with bounded concurrency

For each requested repository/family pair, the collector will request every page of open alerts before producing output. Repository/family pairs will be processed with a small fixed concurrency limit rather than fully sequentially or unboundedly in parallel.

Why:
- Summary counts are incorrect if pagination is ignored.
- A small fixed concurrency limit keeps latency reasonable on teams with dozens of repositories.
- Avoiding a user-facing concurrency flag keeps the MVP simpler while still documenting the intended performance shape.

Alternative considered:
- Fully sequential collection.
  Rejected because large teams would experience avoidable latency.
- Unbounded parallel collection.
  Rejected because it needlessly increases burstiness and rate-limit risk.

### 8. "Open" means API `state=open`

The collector will request only API items whose state is exactly `open`.

Why:
- Dependabot and code scanning expose multiple non-open states such as `fixed`, `dismissed`, and `auto_dismissed`.
- Pinning the state avoids re-litigating whether recently fixed or dismissed items belong in the MVP output.

Alternative considered:
- Infer openness from missing dismissal/fixed timestamps.
  Rejected because the APIs already provide an explicit state filter.

## Risks / Trade-offs

- [Large repository sets increase API volume] → Keep the MVP narrow, use bounded concurrency instead of unbounded fanout, and surface rate-limit reset times through existing CLI error translation.
- [Repository-level feature availability differs across repositories] → Treat unavailable features as zero-alert results and reserve warnings for true access failures.
- [Alert families do not share identical fields] → Standardize on a small common TSV projection instead of trying to expose every native API field in v1.
- [Users may expect secret scanning because it is part of GitHub security] → Explicitly document that the maintainer baseline excludes secret scanning in the MVP.
- [The maintainer baseline does not hold for `--ownership=codeowners`] → Document the gap, keep traversal best-effort, and require a non-zero exit when access failures make the result incomplete.

## Migration Plan

No data migration is required.

Rollout steps:
1. Add the new command tree and collectors behind the existing root.
2. Add README/help updates describing the maintainer baseline and supported alert families.
3. Release as a normal additive extension update.

Rollback is straightforward: revert the added command files and internal collector package if the command surface needs to be removed before broad rollout.

## Open Questions

- Whether `alerts` should remain TSV-only in the initial release or grow a JSON mode in a follow-up change.
- Whether repeated per-repository access failures should be summarized at the end in addition to immediate stderr warnings.
