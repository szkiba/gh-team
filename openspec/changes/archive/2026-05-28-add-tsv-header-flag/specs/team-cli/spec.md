## ADDED Requirements

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
