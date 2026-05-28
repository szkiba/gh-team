## ADDED Requirements

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

## MODIFIED Requirements

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
