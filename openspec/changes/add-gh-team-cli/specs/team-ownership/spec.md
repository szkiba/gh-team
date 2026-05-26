# team-ownership Specification

## ADDED Requirements

### Requirement: Permission-based ownership resolution
The system SHALL resolve the set of repositories owned by a team using GitHub team-to-repository permission assignments when `--ownership=permission` (the default) is in effect. A repository is owned if the named team, or any of its sub-teams, has the `Admin` or `Maintain` permission on that repository, matching the `permission` field GitHub exposes on the team-to-repo binding.

#### Scenario: Direct admin permission
- **GIVEN** team `octo/platform` has the `Admin` permission on repository `octo/api`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=permission`
- **THEN** repository `octo/api` is included in the result

#### Scenario: Sub-team maintain permission
- **GIVEN** team `octo/platform` has a sub-team `octo/platform-ingest`
- **AND** `octo/platform-ingest` has the `Maintain` permission on repository `octo/ingestor`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=permission`
- **THEN** repository `octo/ingestor` is included in the result

#### Scenario: Insufficient permission excluded
- **GIVEN** team `octo/platform` only has the `Write` (push) permission on repository `octo/contrib`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=permission`
- **THEN** repository `octo/contrib` is NOT included in the result

#### Scenario: Direct-only excludes sub-teams
- **GIVEN** sub-team `octo/platform-ingest` has the `Admin` permission on `octo/ingestor`
- **AND** parent team `octo/platform` has no permission on `octo/ingestor`
- **WHEN** the resolver is invoked with `--direct-only --ownership=permission`
- **THEN** repository `octo/ingestor` is NOT included in the result

### Requirement: CODEOWNERS ownership resolution
When `--ownership=codeowners` is in effect, the system SHALL resolve ownership through a two-step pipeline: candidate discovery via GitHub code search, followed by exact verification via fetch-and-parse of the effective `CODEOWNERS` file. The decision to include or exclude a *candidate* repository SHALL come exclusively from parsing the fetched file; the code search result is a candidate filter only and SHALL NOT, on its own, establish ownership.

**Completeness scope.** The system makes an **exact per-candidate** decision but does NOT guarantee org-wide completeness of the result set, because the candidate set comes from GitHub's code search index, which lags the default branch. A repository that legitimately satisfies the ownership rule may be absent from the result set if its `CODEOWNERS` file is not yet (or no longer) reflected in the search index. The command SHALL print a one-line note to stderr identifying this limitation whenever `--ownership=codeowners` is used, so users are not misled into treating the output as a guaranteed-complete enumeration.

**Candidate discovery.** The system SHALL issue a GitHub code search query of the form `org:<ORG> path:CODEOWNERS "@<ORG>/<TEAM>"`. The broad query (just the team mention) is required so that wildcard lines with multiple owners or unusual whitespace are not missed at the discovery step. Repositories returned by this search are *candidates*; nothing further is concluded from the snippets.

**Effective file selection.** For each candidate, the system SHALL fetch the contents of the **first existing** of the following paths on the default branch, in this order, and use that file alone:

1. `.github/CODEOWNERS`
2. `CODEOWNERS`
3. `docs/CODEOWNERS`

Later paths SHALL be ignored once an earlier path is found, mirroring GitHub's own resolution order. If none of the three paths exists, the candidate SHALL be rejected.

**Parsing.** For each line of the effective file the system SHALL:

1. Discard everything from the first `#` character on the line to end-of-line (comment stripping).
2. Skip the line if the remaining text is blank.
3. Otherwise tokenize the remaining text on whitespace: the first token is the pattern, the rest are owners.

**Ownership rule.** The repository SHALL be considered owned by the team if, and only if, the **last** parsed line whose first token is exactly `*` (the bare wildcard, one character) lists `@<ORG>/<TEAM>` as one of its owners. The team-slug comparison SHALL be case-insensitive, matching GitHub's behavior. Owner position on the line is irrelevant: a line like `* @octo/security @octo/platform` establishes ownership the same as `* @octo/platform @octo/security`.

This rule respects CODEOWNERS' last-matching-pattern precedence: an earlier wildcard line that names the team is overridden by any later wildcard line that does not. Path-scoped patterns (e.g. `/docs/*`, `*.md`, `apps/*`) do NOT establish ownership in the MVP.

#### Scenario: Wildcard owned by team
- **GIVEN** candidate repository `octo/api` has `.github/CODEOWNERS` containing exactly the line `* @octo/platform`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=codeowners`
- **THEN** repository `octo/api` is included in the result

#### Scenario: Team is one of multiple owners on the wildcard line
- **GIVEN** candidate repository `octo/api` has `CODEOWNERS` containing exactly the line `* @octo/security @octo/platform`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=codeowners`
- **THEN** repository `octo/api` is included in the result

#### Scenario: Later wildcard line overrides earlier one
- **GIVEN** candidate repository `octo/api` has `CODEOWNERS` containing, in order, `* @octo/platform` and later `* @octo/other`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=codeowners`
- **THEN** repository `octo/api` is NOT included in the result, because the later `*` line supersedes the earlier one

#### Scenario: Path-scoped entry only
- **GIVEN** candidate repository `octo/api` has `CODEOWNERS` containing only `/docs/ @octo/platform`
- **AND** has no `*` line naming `@octo/platform`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=codeowners`
- **THEN** repository `octo/api` is NOT included in the result

#### Scenario: Team mentioned only in a comment
- **GIVEN** candidate repository `octo/api` has `CODEOWNERS` containing `# fallback owner used to be * @octo/platform` and an active `* @octo/other` line
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=codeowners`
- **THEN** repository `octo/api` is NOT included in the result

#### Scenario: File at `.github/CODEOWNERS` takes precedence over root
- **GIVEN** candidate repository `octo/api` has both `.github/CODEOWNERS` containing `* @octo/other` and `CODEOWNERS` at the repo root containing `* @octo/platform`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=codeowners`
- **THEN** the resolver uses `.github/CODEOWNERS` only
- **AND** repository `octo/api` is NOT included in the result, even though the root file would have established ownership

#### Scenario: Fallback to `docs/CODEOWNERS`
- **GIVEN** candidate repository `octo/api` has no `.github/CODEOWNERS` and no root `CODEOWNERS`, but has `docs/CODEOWNERS` containing `* @octo/platform`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=codeowners`
- **THEN** repository `octo/api` is included in the result

#### Scenario: Candidate with no CODEOWNERS at any path
- **GIVEN** code search returned candidate repository `octo/stale` (the file existed at the time of indexing but has since been removed) and none of the three candidate paths exist on the default branch
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=codeowners`
- **THEN** repository `octo/stale` is NOT included in the result

#### Scenario: Search-index lag note shown
- **WHEN** the user invokes any subcommand with `--ownership=codeowners`
- **THEN** stderr contains a one-line note that the result is based on GitHub's code search index and may omit recently added or renamed `CODEOWNERS` files until they are re-indexed
- **AND** the exit status, stdout, and the substance of the result are not changed by this note

### Requirement: Reject incompatible flag combination
The system SHALL reject `--direct-only` combined with `--ownership=codeowners` at the root command, before any API call is made, exiting with a non-zero status and printing an error to stderr. CODEOWNERS has no team hierarchy to limit.

#### Scenario: Conflicting flags
- **WHEN** the user invokes any subcommand with `--direct-only --ownership=codeowners`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that `--direct-only` is only valid with `--ownership=permission`

### Requirement: Archived repositories excluded by default
The resolver SHALL exclude archived repositories from results unless `--include-archived` is set. This applies to both ownership strategies.

#### Scenario: Archived repo dropped by default
- **GIVEN** repository `octo/legacy` is archived and owned by team `octo/platform` under the active strategy
- **WHEN** the resolver is invoked without `--include-archived`
- **THEN** repository `octo/legacy` is NOT included in the result

#### Scenario: Archived repo included on opt-in
- **GIVEN** the same archived repository `octo/legacy`
- **WHEN** the resolver is invoked with `--include-archived`
- **THEN** repository `octo/legacy` IS included in the result

### Requirement: Deterministic ordering
The resolver SHALL return repositories sorted alphabetically by full repository name (`<org>/<repo>`), with duplicates removed. This guarantees stable output for scripting.

#### Scenario: Same repo reachable via parent and sub-team
- **GIVEN** repository `octo/api` is reachable via both team `octo/platform` and sub-team `octo/platform-api`
- **WHEN** the resolver is invoked for `octo/platform` with `--ownership=permission`
- **THEN** `octo/api` appears exactly once in the result
