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

### Requirement: Security PR listing subcommand
The system SHALL provide a `gh team security prs <team>` subcommand that lists open pull requests in the team-owned repository set whose title or labels match security signals. The command SHALL accept the same team argument form (`<org>/<team-slug>`) and root flags as the other `security` subcommands and SHALL traverse the same owned-repository set produced by the ownership resolver.

The command SHALL only consider pull requests in state `open`. There is no flag to broaden the state set in v1.

#### Scenario: Lists open security-tagged PRs across owned repositories
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web`
- **AND** `octo/api` has one open PR `#17` titled `[security] bump openssl` and one open PR `#23` titled `unrelated refactor`
- **AND** `octo/web` has one open PR `#4` titled `routine readme tweak` with label `security`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout contains rows for `octo/api#17` and `octo/web#4`
- **AND** stdout does not contain a row for `octo/api#23`
- **AND** the exit status is 0

#### Scenario: Closed and merged PRs are excluded
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one merged PR titled `[security] fix CVE-2024-0001`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout contains no row for the merged PR
- **AND** the exit status is 0

### Requirement: Default signals match common security conventions
When neither `--title` nor `--label` is set, the system SHALL apply the following defaults and SHALL include a PR in the result when at least one signal matches (OR-combined):

- title default regex: `(?i)^\[security\]|^security:|\[security\]$`
- label default: a PR has a label whose name is exactly `security`

The defaults SHALL be replaced individually when their corresponding override is set. If `--title <regex>` is set, the title default is dropped. If one or more `--label <l>` flags are set, the label default is dropped and only the user-supplied labels apply. A label override SHALL be repeatable and SHALL match when a PR has at least one label equal to any of the supplied values.

#### Scenario: Title-only match catches `[security]` prefix
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#5` titled `[security] rotate keys` with no labels
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout contains a row for `octo/api#5`

#### Scenario: Title-only match catches `[Security]` suffix case-insensitively
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#6` titled `rotate keys [Security]` with no labels
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout contains a row for `octo/api#6`

#### Scenario: Label-only match catches the `security` label
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#7` titled `routine bump` with labels `dependencies, security`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout contains a row for `octo/api#7`

#### Scenario: PR matching neither signal is excluded
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#8` titled `add dark mode` with label `feature`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout contains no row for `octo/api#8`

#### Scenario: --title override replaces the default regex
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has open PRs `#9` titled `[security] rotate keys` and `#10` titled `SEC-1234 rotate keys`
- **WHEN** the user runs `gh team security prs octo/platform --title '^SEC-[0-9]+'`
- **THEN** stdout contains a row for `octo/api#10`
- **AND** stdout contains no row for `octo/api#9` unless its label set also matches the label default
- **AND** the label default still applies to other PRs

#### Scenario: --label override replaces the default label and is repeatable
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has open PRs `#11` with label `security`, `#12` with label `compliance`, and `#13` with label `audit`
- **WHEN** the user runs `gh team security prs octo/platform --label compliance --label audit`
- **THEN** stdout contains rows for `octo/api#12` and `octo/api#13`
- **AND** stdout contains no row for `octo/api#11` unless its title matches the title default

### Requirement: Default-mode row shape and field-name contract
In default mode (no output flag set), the command SHALL emit exactly one line per matching pull request containing only the pull request's `html_url` followed by a single `\n`. Default mode SHALL NOT emit a header, a column-joined TSV row, or any other text on stdout. Rows SHALL remain sorted by `repo` ascending, then by `number` descending (see ordering requirement).

When `--header` is set, the command SHALL widen each row to a tab-separated line with exactly seven columns in this order: `repo`, `number`, `state`, `title`, `author`, `updated`, `url` (header-mode row shape). Field semantics under `--header`:

- `repo` is `<owner>/<name>`.
- `number` is the pull request number rendered as a decimal integer.
- `state` is the literal string `open` in v1.
- `title` is the pull request title with `\t` and `\n` replaced by a single space (sanitization rule).
- `author` is the pull request author's GitHub login. For app-authored PRs (for example `dependabot[bot]`), the login SHALL be emitted verbatim including the `[bot]` suffix.
- `updated` is the pull request's `updated_at` timestamp rendered in ISO-8601 UTC with a `Z` suffix (for example `2026-05-28T07:30:15Z`).
- `url` is the pull request's `html_url`.

The JSON and template field-name contract SHALL use exactly these seven names: `.repo`, `.number`, `.state`, `.title`, `.author`, `.updated`, `.url`. In `--json` mode, `.title` SHALL preserve the original title verbatim without the tab / newline sanitization applied by `--header` mode. `.number` SHALL be a JSON integer.

#### Scenario: Default row exposes only the PR URL
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#42` titled `[security] rotate keys`, authored by `alice`, with label `security`, last updated at `2026-05-28T07:30:15Z`, at `https://github.com/octo/api/pull/42`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout is exactly `https://github.com/octo/api/pull/42\n`
- **AND** the exit status is 0

#### Scenario: --header widens to the seven-column row contract
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#42` titled `[security] rotate keys`, authored by `alice`, with label `security`, last updated at `2026-05-28T07:30:15Z`, at `https://github.com/octo/api/pull/42`
- **WHEN** the user runs `gh team security prs octo/platform --header`
- **THEN** stdout contains the line `octo/api\t42\topen\t[security] rotate keys\talice\t2026-05-28T07:30:15Z\thttps://github.com/octo/api/pull/42` after the header line
- **AND** the exit status is 0

#### Scenario: Title with embedded tab is sanitized under --header
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#1` titled `[security]\tweird\ttitle`, last updated at `2026-05-28T07:30:15Z`
- **WHEN** the user runs `gh team security prs octo/platform --header`
- **THEN** the `title` cell in stdout contains `[security] weird title` with single spaces in place of the tabs
- **AND** the row still contains exactly seven tab-separated columns

#### Scenario: --json preserves the original title verbatim
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#1` titled `[security]\tweird` with an embedded tab
- **WHEN** the user runs `gh team security prs octo/platform --json`
- **THEN** stdout is a JSON array whose first element has a `.title` field equal to the original title with the embedded tab preserved
- **AND** the JSON output is parseable

#### Scenario: Default mode emits nothing besides URLs
- **GIVEN** team `octo/platform` owns `octo/api` with two matching open PRs `#23` and `#17`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout contains exactly two lines, each a single PR URL terminated by `\n`
- **AND** stdout contains no tab characters and no header line

### Requirement: Result ordering
The system SHALL sort rows by `repo` ascending, then by `number` descending so the newest pull request from each repository appears first within that repository's group. This ordering SHALL apply to default mode, `--header`, `--json`, and `--template` output identically.

#### Scenario: Multiple PRs from the same repo sort newest-first within the repo
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web`
- **AND** `octo/api` has open security-matching PRs `#17` and `#23`
- **AND** `octo/web` has open security-matching PR `#4`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout rows appear in this order: `octo/api#23`, `octo/api#17`, `octo/web#4`

### Requirement: Header line for security PR listing
When `--header` is set and default (TSV) output mode is in effect, the system SHALL emit exactly one tab-separated header line `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl` before any data rows. The data rows that follow SHALL match the seven-column TSV shape described in the row-shape requirement. The header line SHALL be emitted even when zero pull requests match.

When `--header` is NOT set, `security prs` default mode SHALL emit only PR URLs (one per line) and SHALL NOT emit a header line.

#### Scenario: Header line precedes data rows
- **GIVEN** team `octo/platform` owns `octo/api` with one matching open PR
- **WHEN** the user runs `gh team security prs octo/platform --header`
- **THEN** stdout's first line is exactly `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl`
- **AND** the matching pull request's seven-column data row follows

#### Scenario: Header still emits on empty result
- **GIVEN** team `octo/empty` owns no repositories with matching pull requests
- **WHEN** the user runs `gh team security prs octo/empty --header`
- **THEN** stdout is exactly `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl\n`
- **AND** the exit status is 0

#### Scenario: Default mode (no --header) emits no header
- **GIVEN** team `octo/platform` owns `octo/api` with one matching open PR at `https://github.com/octo/api/pull/42`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout is exactly `https://github.com/octo/api/pull/42\n`
- **AND** no header line appears in stdout

### Requirement: Invalid title regex is rejected before any repository call
When `--title` is set to a value that fails `regexp.Compile`, the system SHALL exit non-zero before issuing any pull request request and SHALL print an error to stderr that names the offending pattern and the underlying compile error. Stdout SHALL be empty.

#### Scenario: Bad title regex fails fast
- **WHEN** the user runs `gh team security prs octo/platform --title '['`
- **THEN** the command exits with a non-zero status
- **AND** stderr names the invalid pattern `[` and the regex compile error
- **AND** stdout is empty
- **AND** no pull request request is issued

### Requirement: Per-repository failures follow the partial-failure pattern
When the system cannot list pull requests for a particular owned repository (HTTP 403, 404, or other non-fatal error), the system SHALL continue processing the remaining repositories, SHALL print a warning to stderr naming the affected repository and the failure category, and SHALL exit with status `1` after rendering any successful results. Successful rows SHALL still be printed in the documented order. The partial-failure rendering rule applies identically in default mode (URL-only lines), `--header` mode (header + seven-column rows), `--json`, and `--template` modes.

#### Scenario: Partial failure still renders successful URL rows in default mode
- **GIVEN** team `octo/platform` owns `octo/api` and `octo/web`
- **AND** pull request listing succeeds for `octo/api` with one matching PR at `https://github.com/octo/api/pull/42`
- **AND** pull request listing fails for `octo/web` with HTTP 403
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout is exactly `https://github.com/octo/api/pull/42\n`
- **AND** stderr includes a warning naming `octo/web` and the access-denied category
- **AND** the command exits with status `1`

#### Scenario: Partial failure still renders successful rows under --header
- **GIVEN** team `octo/platform` owns `octo/api` and `octo/web`
- **AND** pull request listing succeeds for `octo/api` with one matching row
- **AND** pull request listing fails for `octo/web` with HTTP 403
- **WHEN** the user runs `gh team security prs octo/platform --header`
- **THEN** stdout begins with the `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl` header line followed by the matching seven-column row from `octo/api`
- **AND** stderr includes a warning naming `octo/web` and the access-denied category
- **AND** the command exits with status `1`
