## MODIFIED Requirements

### Requirement: Default-mode header line is opt-in
Data-emitting `gh team` subcommands SHALL accept an optional `--header` boolean flag. When `--header` is set and default (TSV) output mode is in effect, the command SHALL emit a single tab-separated header line of field names before any data rows. Field names SHALL match the JSON and template field-name contract published for each command. When `--header` is not set, command stdout SHALL remain byte-for-byte unchanged from that command's existing no-flag default-mode contract.

For commands whose default-mode rows already carry every field named in the header (`security summary`, `security alerts`), `--header` SHALL leave the rows themselves unchanged. For `gh team repo list` and `gh team security prs`, whose default mode emits one column, `--header` SHALL also widen each data row so that the columns under the header line match the header — the precise per-command shape is defined in the `team-repo` and `team-security` specs.

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

#### Scenario: --header widens security prs from URL-only to seven-column TSV
- **GIVEN** team `octo/platform` owns `octo/api` with one matching open PR
- **WHEN** the user runs `gh team security prs octo/platform --header`
- **THEN** stdout's first line is exactly `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl`
- **AND** the matching pull request's data row that follows is a seven-column tab-separated row, not the URL-only line the no-flag default would produce

### Requirement: `security prs` honors the shared output-flag contract
The `gh team security prs` subcommand SHALL accept the same output flag set as the other data-emitting subcommands defined in this capability: `--header`, `--json`, and `--template`. The system SHALL reject `--header` when combined with `--json` or `--template`, and SHALL reject `--json` and `--template` together. Rejection SHALL exit non-zero with stderr explaining which flags conflicted; stdout SHALL be empty in the rejection case.

When no output flag is set, default-mode output SHALL apply with the single-URL-per-line shape defined in the `team-security` capability. When `--header` is set, default-mode output SHALL widen to the seven-column TSV shape defined in the `team-security` capability, prefixed by the header line.

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
