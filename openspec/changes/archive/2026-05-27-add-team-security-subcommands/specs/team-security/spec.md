## ADDED Requirements

### Requirement: Summarize open security alerts for owned repositories
The system SHALL provide a `gh team security summary <org/team-slug>` command that resolves the repositories owned by the team under the active ownership strategy and prints a tab-separated summary of open security alerts across those repositories. For this command, "open" SHALL mean API items returned with `state=open`. Each output line SHALL contain `<org>/<repo>`, the alert family, and the count of open alerts for that family. The command SHALL emit only lines whose open-alert count is greater than zero, sort output first by full repository name and then by alert family, and print no header or decoration.

#### Scenario: Summary across multiple repositories
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web`
- **AND** `octo/api` has 2 open Dependabot alerts and 1 open code-scanning alert
- **AND** `octo/web` has 3 open code-scanning alerts
- **WHEN** the user runs `gh team security summary octo/platform`
- **THEN** stdout is exactly:
  ```
  octo/api	code-scanning	1
  octo/api	dependabot	2
  octo/web	code-scanning	3
  ```
- **AND** the exit status is 0

#### Scenario: Empty summary
- **GIVEN** team `octo/platform` owns repositories but none of them have open alerts in the requested families
- **WHEN** the user runs `gh team security summary octo/platform`
- **THEN** stdout is empty
- **AND** the exit status is 0

### Requirement: List individual open security alerts for owned repositories
The system SHALL provide a `gh team security alerts <org/team-slug>` command that resolves the repositories owned by the team under the active ownership strategy and prints one tab-separated line per open alert. Each line SHALL contain the alert family, full repository name, a stable family-specific key, the family-specific severity value, and the alert HTML URL. For Dependabot alerts, the key SHALL be `<ecosystem>:<package>@<manifest-path>`. For code-scanning alerts, the key SHALL be the rule ID. For code-scanning severity, the command SHALL use `security_severity_level` when present and fall back to `rule.severity` otherwise. Output SHALL be sorted by repository name, alert family, key, and URL, with no header or decoration.

#### Scenario: Mixed alert listing
- **GIVEN** team `octo/platform` owns repository `octo/api`
- **AND** `octo/api` has an open Dependabot alert for package `lodash` in ecosystem `npm` and manifest `/web/package-lock.json` with severity `high` at `https://github.com/octo/api/security/dependabot/7`
- **AND** `octo/api` has an open code-scanning alert for rule `go/sql-injection` with security severity `high` at `https://github.com/octo/api/code-scanning/4`
- **WHEN** the user runs `gh team security alerts octo/platform`
- **THEN** stdout is exactly:
  ```
  code-scanning	octo/api	go/sql-injection	high	https://github.com/octo/api/code-scanning/4
  dependabot	octo/api	npm:lodash@/web/package-lock.json	high	https://github.com/octo/api/security/dependabot/7
  ```
- **AND** the exit status is 0

#### Scenario: Alert listing respects ownership filters
- **GIVEN** repository `octo/legacy` is archived and has open Dependabot alerts
- **AND** team `octo/platform` owns `octo/legacy`
- **WHEN** the user runs `gh team security alerts octo/platform` without `--include-archived`
- **THEN** no alert from `octo/legacy` appears in stdout

### Requirement: Select supported alert families
Security commands SHALL support a `--kind=dependabot|code-scanning|all` flag, with `all` as the default. `all` SHALL mean exactly the union of `dependabot` and `code-scanning`. The system SHALL reject any other value with exit status `1` and an error naming the supported values. Secret scanning SHALL NOT be included in `all` for this change, and later alert families SHALL require explicit opt-in by name unless a future compatibility change updates this alias.

#### Scenario: Dependabot-only summary
- **GIVEN** team `octo/platform` owns repository `octo/api`
- **AND** `octo/api` has both open Dependabot and open code-scanning alerts
- **WHEN** the user runs `gh team security summary octo/platform --kind=dependabot`
- **THEN** stdout contains only the Dependabot summary line for `octo/api`

#### Scenario: Unsupported alert family rejected
- **WHEN** the user runs `gh team security summary octo/platform --kind=secret-scanning`
- **THEN** the command exits with status `1`
- **AND** stderr explains that the supported values are `dependabot`, `code-scanning`, and `all`

### Requirement: Collect every alert page before rendering output
For each resolved repository and requested alert family, the system SHALL collect every page of alerts returned by the GitHub API before rendering summary counts or alert lines. The system SHALL request only alerts with `state=open` for both supported alert families.

#### Scenario: Multi-page Dependabot summary
- **GIVEN** team `octo/platform` owns repository `octo/api`
- **AND** the open Dependabot alerts for `octo/api` span multiple API pages
- **WHEN** the user runs `gh team security summary octo/platform --kind=dependabot`
- **THEN** the summary count for `octo/api` includes alerts from every page

#### Scenario: Multi-page code-scanning listing
- **GIVEN** team `octo/platform` owns repository `octo/api`
- **AND** the open code-scanning alerts for `octo/api` span multiple API pages
- **WHEN** the user runs `gh team security alerts octo/platform --kind=code-scanning`
- **THEN** stdout contains alert lines from every page in deterministic sorted order

### Requirement: Continue across repositories and report hard failures
Security commands SHALL collect alerts repository-by-repository after ownership resolution. If a requested alert family is unavailable or disabled for a repository, the system SHALL treat that repository/family pair as contributing zero alerts and SHALL continue without a warning. If a repository/family pair cannot be queried because the authenticated user lacks required access or token scope, the system SHALL continue processing the remaining repositories, print a warning to stderr naming the affected repository and alert family, and exit with status `1` after processing completes.

#### Scenario: Feature unavailable is treated as zero alerts
- **GIVEN** team `octo/platform` owns repository `octo/api`
- **AND** code scanning is not enabled for `octo/api`
- **WHEN** the user runs `gh team security summary octo/platform --kind=code-scanning`
- **THEN** `octo/api` contributes no output line
- **AND** the command exits with status 0

#### Scenario: One repository lacks alert access
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web`
- **AND** the authenticated user can read Dependabot alerts for `octo/api`
- **AND** the authenticated user cannot read Dependabot alerts for `octo/web`
- **WHEN** the user runs `gh team security alerts octo/platform --kind=dependabot`
- **THEN** stdout still includes any alert lines collected from `octo/api`
- **AND** stderr includes a warning naming `octo/web` and `dependabot`
- **AND** the command exits with status `1` after all repositories have been processed

#### Scenario: Codeowners ownership can surface inaccessible repositories
- **GIVEN** team `octo/platform` owns repository `octo/docs` under `--ownership=codeowners`
- **AND** the authenticated user does not have sufficient repository access to read alerts for `octo/docs`
- **WHEN** the user runs `gh team security summary octo/platform --ownership=codeowners`
- **THEN** stderr includes a warning naming `octo/docs`
- **AND** the command exits with status `1` after any accessible repositories have been processed
