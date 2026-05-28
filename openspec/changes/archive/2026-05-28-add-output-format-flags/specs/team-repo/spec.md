## MODIFIED Requirements

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
