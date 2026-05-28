# team-repo Specification

## Purpose
TBD - created by archiving change add-gh-team-cli. Update Purpose after archive.
## Requirements
### Requirement: List owned repositories
The system SHALL provide a `gh team repo list <org/team-slug>` command that prints the full names of all repositories owned by the team under the active ownership strategy and global filters.

In default mode (no output flag), output SHALL remain one full repository name per line, sorted alphabetically, with no headers, counts, or decoration.

When `--json` is set, stdout SHALL be a JSON array of repository objects sorted the same way as default mode. Each object SHALL contain at least:
- `owner`
- `name`
- `full_name`
- `archived`

When `--template <go-template>` is set, the command SHALL render one line per repository using the template context fields:
- `.owner`
- `.name`
- `.full_name`
- `.archived`

Items SHALL be rendered in the same deterministic sorted order as default mode (alphabetical by full repository name).

#### Scenario: Default output remains unchanged
- **GIVEN** team `octo/platform` owns repositories `octo/api`, `octo/web`, `octo/ingestor`
- **WHEN** the user runs `gh team repo list octo/platform`
- **THEN** stdout is exactly:
  ```
  octo/api
  octo/ingestor
  octo/web
  ```
- **AND** the exit status is 0

#### Scenario: JSON output for repo list
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web`
- **WHEN** the user runs `gh team repo list octo/platform --json`
- **THEN** stdout is valid JSON array output with two items
- **AND** each item contains `owner`, `name`, `full_name`, and `archived`

#### Scenario: Template output for repo list
- **GIVEN** team `octo/platform` owns repository `octo/api`
- **WHEN** the user runs `gh team repo list octo/platform --template '{{.name}}'`
- **THEN** stdout is exactly:
  ```
  api
  ```
- **AND** the exit status is 0

#### Scenario: Template output preserves default ordering
- **GIVEN** team `octo/platform` owns repositories `octo/web`, `octo/api`, `octo/ingestor`
- **WHEN** the user runs `gh team repo list octo/platform --template '{{.full_name}}'`
- **THEN** stdout is exactly:
  ```
  octo/api
  octo/ingestor
  octo/web
  ```

#### Scenario: Empty result
- **GIVEN** team `octo/empty` owns no repositories under the active strategy and filters
- **WHEN** the user runs `gh team repo list octo/empty`
- **THEN** stdout is empty
- **AND** the exit status is 0

#### Scenario: Listing with CODEOWNERS strategy
- **GIVEN** team `octo/platform` owns `octo/api` via CODEOWNERS but `octo/web` only via the `permission` strategy
- **WHEN** the user runs `gh team repo list octo/platform --ownership=codeowners`
- **THEN** stdout is exactly `octo/api`

### Requirement: Clone owned repositories
The system SHALL provide a `gh team repo clone <org/team-slug>` command that clones each owned repository into a subdirectory of the current working directory by delegating to `gh repo clone <org>/<repo>`. The system SHALL NOT invoke `git` directly. The system SHALL process repositories in the same alphabetical order as `repo list`.

#### Scenario: Happy path
- **GIVEN** team `octo/platform` owns `octo/api` and `octo/web`
- **WHEN** the user runs `gh team repo clone octo/platform` in an empty directory
- **THEN** `gh repo clone octo/api` and `gh repo clone octo/web` are each invoked exactly once
- **AND** the exit status is 0

#### Scenario: Skip already-cloned directories
- **GIVEN** team `octo/platform` owns `octo/api` and `octo/web`
- **AND** a directory named `api` already exists in the current working directory
- **WHEN** the user runs `gh team repo clone octo/platform`
- **THEN** `gh repo clone octo/api` is NOT invoked
- **AND** a non-fatal warning is printed to stderr indicating `api` was skipped
- **AND** `gh repo clone octo/web` is still invoked
- **AND** the exit status is 0

#### Scenario: Aggregate clone failures
- **GIVEN** team `octo/platform` owns `octo/api`, `octo/web`, `octo/legacy`
- **AND** the clone of `octo/web` fails
- **WHEN** the user runs `gh team repo clone octo/platform`
- **THEN** the other two clones are still attempted
- **AND** a summary of failures is written to stderr
- **AND** the exit status is non-zero

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

