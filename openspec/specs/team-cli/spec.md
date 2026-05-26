# team-cli Specification

## Purpose
TBD - created by archiving change add-gh-team-cli. Update Purpose after archive.
## Requirements
### Requirement: Team-slug argument format
Every subcommand that takes a team identifier SHALL require a single positional argument of the form `<org>/<team-slug>`. The system SHALL reject any argument that is missing the `/`, contains more than one `/`, has an empty org part, or has an empty team-slug part. On rejection the system SHALL exit with a non-zero status and print a usage hint showing the expected form.

#### Scenario: Missing slash
- **WHEN** the user runs `gh team repo list octo-platform`
- **THEN** the command exits with a non-zero status
- **AND** stderr contains a message that includes the expected form `<org>/<team-slug>`

#### Scenario: Empty org part
- **WHEN** the user runs `gh team repo list /platform`
- **THEN** the command exits with a non-zero status
- **AND** stderr contains a message that includes the expected form `<org>/<team-slug>`

#### Scenario: Empty team-slug part
- **WHEN** the user runs `gh team repo list octo/`
- **THEN** the command exits with a non-zero status
- **AND** stderr contains a message that includes the expected form `<org>/<team-slug>`

#### Scenario: Multiple slashes
- **WHEN** the user runs `gh team repo list octo/platform/api`
- **THEN** the command exits with a non-zero status
- **AND** stderr contains a message that includes the expected form `<org>/<team-slug>`

### Requirement: Team existence preflight
After argument-format validation and before any ownership resolution begins, the system SHALL verify that the supplied `<org>/<team-slug>` corresponds to a real GitHub team in the named organization, by calling `GET /orgs/<org>/teams/<team-slug>` (or the equivalent GraphQL query). If GitHub returns 404, the system SHALL exit with a non-zero status and print an error to stderr that names the missing team. The check SHALL be performed identically regardless of which `--ownership` strategy is selected, so that all strategies give consistent behavior for stale or mistyped team slugs.

The verified team metadata MAY be cached by the resolver for the rest of the invocation (for example, to seed the sub-team walk in the `permission` strategy) so that the preflight does not add an extra round-trip.

#### Scenario: Team does not exist
- **GIVEN** organization `octo` exists but has no team with slug `deleted-team`
- **WHEN** the user runs `gh team repo list octo/deleted-team` under any `--ownership` setting
- **THEN** the command exits with a non-zero status
- **AND** stderr names the cause as a missing team in the named organization
- **AND** no code search query is issued and no CODEOWNERS file is fetched

#### Scenario: Organization does not exist
- **GIVEN** there is no organization named `nosuchorg` accessible to the authenticated session
- **WHEN** the user runs `gh team repo list nosuchorg/platform`
- **THEN** the command exits with a non-zero status
- **AND** stderr names the cause as a missing organization or team

#### Scenario: Codeowners and permission strategies behave consistently
- **GIVEN** team `octo/old-team` no longer exists but its slug still appears in several `CODEOWNERS` files in the org
- **WHEN** the user runs `gh team repo list octo/old-team --ownership=codeowners`
- **THEN** the command exits with a non-zero status with the missing-team error, matching the behavior of `--ownership=permission` for the same input

### Requirement: Authentication errors give actionable guidance
When any GitHub API call fails because the host `gh` session is unauthenticated, the credentials have expired, or the session lacks a scope required by the operation, the system SHALL exit with a non-zero status and print an error message to stderr that names the underlying cause and tells the user how to fix it (e.g. `run \`gh auth login\``, or `run \`gh auth refresh -s read:org\`` for missing scopes). The raw HTTP status code or generic API error text alone SHALL NOT be the only output.

#### Scenario: No active gh session
- **GIVEN** the host `gh` CLI has no active authenticated session
- **WHEN** the user runs any `gh team` subcommand
- **THEN** the command exits with a non-zero status
- **AND** stderr names the cause as missing authentication
- **AND** stderr instructs the user to run `gh auth login`

#### Scenario: Missing org-read scope
- **GIVEN** the host `gh` session is authenticated but lacks the `read:org` scope required to enumerate team-to-repository assignments
- **WHEN** the user runs `gh team repo list octo/platform`
- **THEN** the command exits with a non-zero status
- **AND** stderr names the cause as a missing scope
- **AND** stderr instructs the user to run `gh auth refresh` with the required scope

### Requirement: Rate-limit errors surface reset time
When a GitHub API call fails with HTTP 403 or 429 because a rate limit has been exceeded, the system SHALL exit with a non-zero status and print an error message to stderr that names the affected limit (core REST, GraphQL, or code search) and the absolute UTC time at which the limit resets, as reported by the response headers.

#### Scenario: Code-search rate limit hit
- **GIVEN** the GitHub code search API returns 403 with an `X-RateLimit-Reset` header
- **WHEN** the user runs `gh team repo list octo/platform --ownership=codeowners`
- **THEN** the command exits with a non-zero status
- **AND** stderr identifies the affected limit as code search
- **AND** stderr includes the reset time in UTC

