# team-repo Specification

## Purpose
TBD - created by archiving change add-gh-team-cli. Update Purpose after archive.
## Requirements
### Requirement: List owned repositories
The system SHALL provide a `gh team repo list <org/team-slug>` command that prints the full names of all repositories owned by the team — under the active ownership strategy and global filters — one per line, sorted alphabetically by repository name. The output SHALL contain only repository names, with no headers, counts, or decoration, so it can be piped into other shell commands.

#### Scenario: Default permission strategy listing
- **GIVEN** team `octo/platform` owns repositories `octo/api`, `octo/web`, `octo/ingestor` under the `permission` strategy
- **WHEN** the user runs `gh team repo list octo/platform`
- **THEN** stdout is exactly:
  ```
  octo/api
  octo/ingestor
  octo/web
  ```
- **AND** the exit status is 0

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

