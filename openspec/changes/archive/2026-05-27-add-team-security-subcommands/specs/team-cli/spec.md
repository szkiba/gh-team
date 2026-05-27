## MODIFIED Requirements

### Requirement: Authentication errors give actionable guidance
When any GitHub API call fails because the host `gh` session is unauthenticated, the credentials have expired, or the session lacks a scope required by the operation, the system SHALL exit with a non-zero status and print an error message to stderr that names the underlying cause and tells the user how to fix it. The guidance SHALL name the scope or command that matches the failing command family, such as `run \`gh auth login\`` for missing authentication, `run \`gh auth refresh -s read:org\`` for ownership resolution, or `run \`gh auth refresh -s read:org,security_events\`` for security-alert collection against private repositories from the host `gh` OAuth session. The raw HTTP status code or generic API error text alone SHALL NOT be the only output.

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

#### Scenario: Missing security-events scope
- **GIVEN** the host `gh` session is authenticated but lacks the scope required to list private repository security alerts
- **WHEN** the user runs `gh team security summary octo/platform`
- **THEN** the command exits with a non-zero status
- **AND** stderr names the cause as a missing security-alert scope
- **AND** stderr instructs the user to run `gh auth refresh -s read:org,security_events`

### Requirement: Rate-limit errors surface reset time
When a GitHub API call fails with HTTP 403 or 429 because a rate limit has been exceeded, the system SHALL exit with a non-zero status and print an error message to stderr that names the affected limit (core REST, GraphQL, or code search, or the secondary / abuse-detection limit) and an absolute UTC reset time. The reset time SHALL come from `X-RateLimit-Reset` for primary rate-limit responses and from `Retry-After` (in either delta-seconds or HTTP-date form per RFC 9110) for secondary rate-limit responses. When GitHub returns a secondary rate-limit response with neither header populated, the message SHALL still identify the failure as a secondary rate limit and SHALL state explicitly that the reset time is unavailable, recommending that the caller wait a few minutes before retrying.

#### Scenario: Primary code-search rate limit hit
- **GIVEN** the GitHub code search API returns 403 with `X-RateLimit-Remaining: 0` and an `X-RateLimit-Reset` header
- **WHEN** the user runs `gh team repo list octo/platform --ownership=codeowners`
- **THEN** the command exits with a non-zero status
- **AND** stderr identifies the affected limit as code search
- **AND** stderr includes the reset time in UTC

#### Scenario: Secondary rate limit with Retry-After
- **GIVEN** a security subcommand call returns 403 with a `Retry-After: 60` header but no `X-RateLimit-Reset`
- **WHEN** the user runs `gh team security summary octo/platform`
- **THEN** the command exits with a non-zero status
- **AND** stderr identifies the failure as a secondary rate limit
- **AND** stderr includes an absolute UTC reset time synthesized from the Retry-After delta

#### Scenario: Secondary rate limit without reset metadata
- **GIVEN** a security subcommand call returns 403 with a body mentioning a secondary rate limit but no Retry-After and no X-RateLimit-Reset header
- **WHEN** the user runs `gh team security summary octo/platform`
- **THEN** the command exits with a non-zero status
- **AND** stderr identifies the failure as a secondary rate limit
- **AND** stderr states that the reset time is unavailable and recommends waiting before retrying
