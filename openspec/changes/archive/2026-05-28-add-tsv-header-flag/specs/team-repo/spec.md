## ADDED Requirements

### Requirement: Repo list supports --header in default mode
When `gh team repo list` runs in default mode and `--header` is set, the system SHALL emit exactly one tab-separated header line before any data rows. The header line SHALL be `owner\tname\tfull_name\tarchived` followed by a single `\n`. The data rows that follow SHALL be tab-separated with exactly the four columns named in the header, in the same order: `<owner>\t<name>\t<full_name>\t<archived>`. The `archived` cell SHALL be the lower-case string `true` or `false` so it matches the JSON boolean contract. Rows SHALL remain sorted alphabetically by `full_name` to match every other mode.

When `--header` is NOT set, `repo list` default mode SHALL continue to emit a single `<org>/<repo>` per line as before.

#### Scenario: Header widens repo list rows to the four-column shape
- **GIVEN** team `octo/platform` owns repositories `octo/api` (not archived) and `octo/legacy` (archived)
- **WHEN** the user runs `gh team repo list octo/platform --header --include-archived`
- **THEN** stdout is exactly:
  ```
  owner	name	full_name	archived
  octo	api	octo/api	false
  octo	legacy	octo/legacy	true
  ```
- **AND** the exit status is 0

#### Scenario: Default mode without --header stays single-column
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web`
- **WHEN** the user runs `gh team repo list octo/platform`
- **THEN** stdout is exactly:
  ```
  octo/api
  octo/web
  ```
- **AND** no header line is emitted

#### Scenario: Header emits without data rows
- **GIVEN** team `octo/empty` owns no repositories
- **WHEN** the user runs `gh team repo list octo/empty --header`
- **THEN** stdout is exactly `owner\tname\tfull_name\tarchived\n`
- **AND** the exit status is 0
