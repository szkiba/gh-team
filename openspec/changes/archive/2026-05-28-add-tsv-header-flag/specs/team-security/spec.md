## ADDED Requirements

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
