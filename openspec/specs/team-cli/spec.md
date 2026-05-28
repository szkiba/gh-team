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
When any GitHub API call fails because the host `gh` session is unauthenticated, the credentials have expired, or the session lacks a scope required by the operation, the system SHALL exit with a non-zero status and print an error message to stderr that names the underlying cause and tells the user how to fix it. The guidance SHALL name the scope or command that matches the failing command family, such as `run \`gh auth login\`` for missing authentication, `run \`gh auth refresh -s read:org\`` for ownership resolution, `run \`gh auth refresh -s read:org,security_events\`` for security-alert collection against private repositories from the host `gh` OAuth session, or `run \`gh auth refresh -s repo\`` (or an equivalent fine-grained `Pull requests: read` permission) for the `security prs` subcommand when private-repository pull request enumeration fails because the session lacks repository-read access. The raw HTTP status code or generic API error text alone SHALL NOT be the only output.

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

#### Scenario: Missing repository-read scope for security prs on private repos
- **GIVEN** the host `gh` session is authenticated with `read:org` and can resolve team-to-repository ownership for `octo/platform`
- **AND** the session lacks the classic `repo` scope (or an equivalent fine-grained `Pull requests: read` permission) for the team's private repositories
- **AND** the team owns at least one public repository whose pull-request listing succeeds without `repo` scope
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** the command exits with a non-zero status
- **AND** stderr includes a per-repository warning for each affected private repository naming the missing repository-read scope
- **AND** the warnings instruct the user to run `gh auth refresh -s repo` or to widen the equivalent fine-grained pull-request-read permission
- **AND** the warnings do NOT instruct the user to refresh with `security_events`
- **AND** stdout still contains the matching rows from the accessible public repositories

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

### Requirement: Data-emitting commands support shared output modes
The system SHALL support shared output-mode flags for data-emitting `gh team` subcommands. In this change, the covered commands are `gh team repo list`, `gh team security summary`, and `gh team security alerts`.

The supported output modes are:
- default mode (no output flag): existing line-oriented stdout format.
- JSON mode (`--json`): structured JSON array output.
- template mode (`--template <go-template>`): exactly one rendered line per output item, with embedded newlines treated as an error.

#### Scenario: Shared output mode on a supported command
- **WHEN** the user runs `gh team security summary octo/platform --json`
- **THEN** stdout is valid JSON representing an array of summary items
- **AND** exit status semantics match the command's existing behavior

#### Scenario: Unsupported command remains unchanged
- **WHEN** the user runs `gh team repo clone octo/platform`
- **THEN** the command behavior is unchanged by output-mode features in this change

### Requirement: Output flags are mutually exclusive
The system SHALL reject using `--json` and `--template` together on the same invocation with a non-zero exit status and an error message explaining that only one output mode may be selected.

#### Scenario: Conflicting output flags
- **WHEN** the user runs `gh team repo list octo/platform --json --template '{{.full_name}}'`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that `--json` and `--template` cannot be combined

### Requirement: Template errors are actionable
When template parsing or execution fails, the system SHALL exit with a non-zero status and print an actionable error to stderr that names the template failure. The template engine SHALL be configured with `missingkey=error` so that a reference to a field that does not exist on the command's template context (for example, a typo such as `{{.full_nam}}`) is reported as an execution error rather than rendered as `<no value>`.

#### Scenario: Invalid template syntax
- **WHEN** the user runs `gh team security alerts octo/platform --template '{{.repo'`
- **THEN** the command exits with a non-zero status
- **AND** stderr reports a template parse error

#### Scenario: Unknown template field is rejected
- **WHEN** the user runs `gh team repo list octo/platform --template '{{.full_nam}}'`
- **THEN** the command exits with a non-zero status
- **AND** stderr reports a template execution error naming the missing key
- **AND** stdout does NOT contain a `<no value>` placeholder

### Requirement: Template output is strictly one line per item
When a template-mode command renders an item, the resulting line SHALL contain no embedded newlines. The renderer SHALL append a single trailing newline when the rendered string does not already end with one. If the rendered string contains a newline that is not the final character, the system SHALL fail the entire command with a non-zero exit and an error message identifying the offending item, instead of writing more than one line of stdout for a single input item.

#### Scenario: Trailing newline normalization
- **GIVEN** team `octo/platform` owns repository `octo/api`
- **WHEN** the user runs `gh team repo list octo/platform --template '{{.full_name}}'`
- **THEN** stdout contains exactly one line per repository, each terminated by a single `\n`

#### Scenario: Embedded newline in rendered item is rejected
- **WHEN** the user runs `gh team repo list octo/platform --template '{{printf "%s\n%s" .owner .name}}'`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that the template produced more than one line for a single item
- **AND** stdout does not include a multi-line rendering of any item

### Requirement: Default-mode header line is opt-in
Data-emitting `gh team` subcommands SHALL accept an optional `--header` boolean flag. When `--header` is set and default (TSV) output mode is in effect, the command SHALL emit a single tab-separated header line of field names before any data rows. Field names SHALL match the JSON and template field-name contract published for each command. When `--header` is not set, command stdout SHALL remain byte-for-byte unchanged from the existing default-mode contract.

For commands whose default-mode rows already carry every field named in the header (`security summary`, `security alerts`), `--header` SHALL leave the rows themselves unchanged. For `gh team repo list`, whose default mode emits one column, `--header` SHALL also widen each data row so that the columns under the header line match the header — the precise per-command shape is defined in the `team-repo` and `team-security` specs.

The header line SHALL be emitted even when the result set has zero data rows so that downstream spreadsheet importers can pre-populate column names.

#### Scenario: Header off by default preserves byte contract
- **GIVEN** team `octo/platform` owns repositories `octo/api`, `octo/web`
- **WHEN** the user runs `gh team repo list octo/platform`
- **THEN** stdout is identical to the existing default-mode output: `octo/api\nocto/web\n`
- **AND** no header line is emitted

#### Scenario: Header on prepends one tab-separated field-name line
- **GIVEN** team `octo/platform` owns repository `octo/api` with one open Dependabot alert
- **WHEN** the user runs `gh team security summary octo/platform --header`
- **THEN** stdout's first line is exactly `repo\tfamily\tcount` (tabs between fields, terminated by `\n`)
- **AND** the existing default-mode data rows follow unchanged

#### Scenario: Header still emits on empty result
- **GIVEN** team `octo/empty` owns no repositories under the active strategy and filters
- **WHEN** the user runs `gh team repo list octo/empty --header`
- **THEN** stdout is exactly `owner\tname\tfull_name\tarchived\n`
- **AND** the exit status is 0

### Requirement: Header flag is rejected with non-default output modes
The system SHALL reject `--header` when combined with `--json` or `--template`. Combining `--header` with `--json` would produce a leading non-JSON line and break the JSON-array contract; combining `--header` with `--template` would force a header line into output the user explicitly templated. On rejection the command SHALL exit with a non-zero status and stderr SHALL name both conflicting flags and explain that `--header` applies only to default TSV mode.

#### Scenario: --header conflicts with --json
- **WHEN** the user runs `gh team security alerts octo/platform --header --json`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that `--header` cannot be combined with `--json`
- **AND** stdout is empty

#### Scenario: --header conflicts with --template
- **WHEN** the user runs `gh team repo list octo/platform --header --template '{{.full_name}}'`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that `--header` cannot be combined with `--template`
- **AND** stdout is empty

### Requirement: `security prs` honors the shared output-flag contract
The `gh team security prs` subcommand SHALL accept the same output flag set as the other data-emitting subcommands defined in this capability: `--header`, `--json`, and `--template`. The system SHALL reject `--header` when combined with `--json` or `--template`, and SHALL reject `--json` and `--template` together. Rejection SHALL exit non-zero with stderr explaining which flags conflicted; stdout SHALL be empty in the rejection case.

When no output flag is set, default-mode TSV output SHALL apply with the seven-column shape defined in the `team-security` capability.

#### Scenario: --header conflicts with --json on security prs
- **WHEN** the user runs `gh team security prs octo/platform --header --json`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that `--header` cannot be combined with `--json`
- **AND** stdout is empty

#### Scenario: --header conflicts with --template on security prs
- **WHEN** the user runs `gh team security prs octo/platform --header --template '{{.title}}'`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that `--header` cannot be combined with `--template`
- **AND** stdout is empty

#### Scenario: --json conflicts with --template on security prs
- **WHEN** the user runs `gh team security prs octo/platform --json --template '{{.title}}'`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that `--json` and `--template` are mutually exclusive
- **AND** stdout is empty
