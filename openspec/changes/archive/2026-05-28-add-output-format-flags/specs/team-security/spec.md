## MODIFIED Requirements

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
