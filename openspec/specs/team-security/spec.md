# team-security Specification

## Purpose
TBD - created by archiving change add-team-security-subcommands. Update Purpose after archive.
## Requirements
### Requirement: Summarize open security alerts for owned repositories
The system SHALL provide a `gh team security summary <org/team-slug>` command that resolves the repositories owned by the team under the active ownership strategy and summarizes open alerts.

In default mode (no output flag), each output line SHALL contain `<org>/<repo>`, alert family, and open count as tab-separated fields, sorted by repository then family, with no header.

When `--json` is set, stdout SHALL be a JSON array sorted identically to default mode. Each item SHALL contain:
- `repo`
- `family`
- `count`

When `--template <go-template>` is set, the command SHALL render one line per summary item using template fields:
- `.repo`
- `.family`
- `.count`

Items SHALL be rendered in the same deterministic sorted order as default mode (by repository, then by family).

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

#### Scenario: JSON summary output
- **GIVEN** team `octo/platform` owns repository `octo/api` with open alerts
- **WHEN** the user runs `gh team security summary octo/platform --json`
- **THEN** stdout is valid JSON array output of summary items
- **AND** each item contains `repo`, `family`, and `count`

#### Scenario: Template summary output
- **GIVEN** team `octo/platform` owns repository `octo/api` with one open Dependabot alert
- **WHEN** the user runs `gh team security summary octo/platform --template '{{.repo}} {{.count}}'`
- **THEN** stdout contains one rendered line per summary item

#### Scenario: Template summary preserves default ordering
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web` each with open alerts in both families
- **WHEN** the user runs `gh team security summary octo/platform --template '{{.repo}}/{{.family}}'`
- **THEN** stdout lines appear sorted by repository then by family, identical to the order default mode would produce

### Requirement: List individual open security alerts for owned repositories
The system SHALL provide a `gh team security alerts <org/team-slug>` command that resolves the repositories owned by the team under the active ownership strategy and prints one result item per open alert.

In default mode (no output flag), each line SHALL contain family, repository name, key, severity, and URL as tab-separated fields, sorted by repository, family, key, then URL.

When `--json` is set, stdout SHALL be a JSON array sorted identically to default mode. Each item SHALL contain:
- `family`
- `repo`
- `key`
- `severity`
- `url`

When `--template <go-template>` is set, the command SHALL render one line per alert item using template fields:
- `.family`
- `.repo`
- `.key`
- `.severity`
- `.url`

Items SHALL be rendered in the same deterministic sorted order as default mode (by repository, then family, then key, then URL).

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

#### Scenario: JSON alert output
- **GIVEN** team `octo/platform` owns repository `octo/api` with open alerts
- **WHEN** the user runs `gh team security alerts octo/platform --json`
- **THEN** stdout is valid JSON array output of alert items
- **AND** each item contains `family`, `repo`, `key`, `severity`, and `url`

#### Scenario: Template alert output
- **GIVEN** team `octo/platform` owns repository `octo/api` with an open code-scanning alert
- **WHEN** the user runs `gh team security alerts octo/platform --template '{{.severity}} {{.repo}} {{.url}}'`
- **THEN** stdout contains one rendered line per alert item

#### Scenario: Template alerts preserve default ordering
- **GIVEN** team `octo/platform` owns repositories whose open alerts span both families and multiple rule ids
- **WHEN** the user runs `gh team security alerts octo/platform --template '{{.repo}} {{.family}} {{.key}}'`
- **THEN** stdout lines appear sorted by repository, then family, then key, then URL — identical to the order default mode would produce

### Requirement: Warnings remain on stderr in all output modes
Security commands SHALL keep warnings and partial-failure diagnostics on stderr regardless of selected stdout output mode.

#### Scenario: Partial failure with JSON output
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web`
- **AND** alerts can be read for `octo/api` but not for `octo/web`
- **WHEN** the user runs `gh team security alerts octo/platform --json`
- **THEN** stdout still contains JSON for successful items
- **AND** stderr includes warning text naming `octo/web`
- **AND** the command exits with status `1` after processing completes

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

### Requirement: Security summary supports --header in default mode
When `gh team security summary` runs in default mode and `--header` is set, the system SHALL emit exactly one tab-separated header line before any summary rows. The header line SHALL be `repo\tfamily\tcount` followed by a single `\n`. Data rows that follow SHALL match the existing default-mode contract for `security summary` (tab-separated, sorted by repo then family, lines with zero open alerts omitted). The header line SHALL still emit when no summary rows are produced.

#### Scenario: Summary header content matches v0.3.0 field-name contract
- **GIVEN** team `octo/platform` owns repository `octo/api` with 2 open Dependabot alerts and 1 open code-scanning alert
- **WHEN** the user runs `gh team security summary octo/platform --header`
- **THEN** stdout is exactly:
  ```
  repo	family	count
  octo/api	code-scanning	1
  octo/api	dependabot	2
  ```
- **AND** the exit status is 0

#### Scenario: Summary header emits without data rows
- **GIVEN** team `octo/platform` owns repositories but none have open alerts in the requested families
- **WHEN** the user runs `gh team security summary octo/platform --header`
- **THEN** stdout is exactly `repo\tfamily\tcount\n`
- **AND** the exit status is 0

### Requirement: Security alerts supports --header in default mode
When `gh team security alerts` runs in default mode and `--header` is set, the system SHALL emit exactly one tab-separated header line before any alert rows. The header line SHALL be `family\trepo\tkey\tseverity\turl` followed by a single `\n`. Data rows that follow SHALL match the existing default-mode contract for `security alerts` (tab-separated, sorted by repo / family / key / URL). The header line SHALL still emit when no alert rows are produced.

#### Scenario: Alerts header content matches v0.3.0 field-name contract
- **GIVEN** team `octo/platform` owns repository `octo/api` with one open code-scanning alert for rule `go/sql-injection` at severity `high` at `https://github.com/octo/api/code-scanning/4`
- **WHEN** the user runs `gh team security alerts octo/platform --header`
- **THEN** stdout's first line is exactly `family\trepo\tkey\tseverity\turl\n`
- **AND** the next line is the existing default-mode alert row for that rule
- **AND** the exit status is 0

#### Scenario: Alerts header survives partial failure
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web`
- **AND** alerts can be read for `octo/api` but not for `octo/web`
- **WHEN** the user runs `gh team security alerts octo/platform --header`
- **THEN** stdout begins with the `family\trepo\tkey\tseverity\turl` header line followed by any alert rows collected from `octo/api`
- **AND** stderr includes a warning naming `octo/web`
- **AND** the command exits with status `1` after processing completes
