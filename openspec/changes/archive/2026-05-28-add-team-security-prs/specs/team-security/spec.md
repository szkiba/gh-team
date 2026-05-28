## ADDED Requirements

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
In default mode (no output flag set), the command SHALL emit one tab-separated row per matching pull request with exactly seven columns in this order: `repo`, `number`, `state`, `title`, `author`, `updated`, `url`. Field semantics:

- `repo` is `<owner>/<name>`.
- `number` is the pull request number rendered as a decimal integer.
- `state` is the literal string `open` in v1.
- `title` is the pull request title with `\t` and `\n` replaced by a single space (sanitization rule).
- `author` is the pull request author's GitHub login. For app-authored PRs (for example `dependabot[bot]`), the login SHALL be emitted verbatim including the `[bot]` suffix.
- `updated` is the pull request's `updated_at` timestamp rendered in ISO-8601 UTC with a `Z` suffix (for example `2026-05-28T07:30:15Z`).
- `url` is the pull request's `html_url`.

The JSON and template field-name contract SHALL use exactly these seven names: `.repo`, `.number`, `.state`, `.title`, `.author`, `.updated`, `.url`. In `--json` mode, `.title` SHALL preserve the original title verbatim without the tab / newline sanitization applied by default and `--header` modes. `.number` SHALL be a JSON integer.

#### Scenario: Default row exposes the seven-column contract
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#42` titled `[security] rotate keys`, authored by `alice`, with label `security`, last updated at `2026-05-28T07:30:15Z`, at `https://github.com/octo/api/pull/42`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout contains exactly the line `octo/api\t42\topen\t[security] rotate keys\talice\t2026-05-28T07:30:15Z\thttps://github.com/octo/api/pull/42`

#### Scenario: Title with embedded tab is sanitized in default mode
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#1` titled `[security]\tweird\ttitle`, last updated at `2026-05-28T07:30:15Z`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** the `title` cell in stdout contains `[security] weird title` with single spaces in place of the tabs
- **AND** the row still contains exactly seven tab-separated columns

#### Scenario: --json preserves the original title verbatim
- **GIVEN** team `octo/platform` owns `octo/api`
- **AND** `octo/api` has one open PR `#1` titled `[security]\tweird` with an embedded tab
- **WHEN** the user runs `gh team security prs octo/platform --json`
- **THEN** stdout is a JSON array whose first element has a `.title` field equal to the original title with the embedded tab preserved
- **AND** the JSON output is parseable

### Requirement: Result ordering
The system SHALL sort rows by `repo` ascending, then by `number` descending so the newest pull request from each repository appears first within that repository's group. This ordering SHALL apply to default mode, `--header`, `--json`, and `--template` output identically.

#### Scenario: Multiple PRs from the same repo sort newest-first within the repo
- **GIVEN** team `octo/platform` owns repositories `octo/api` and `octo/web`
- **AND** `octo/api` has open security-matching PRs `#17` and `#23`
- **AND** `octo/web` has open security-matching PR `#4`
- **WHEN** the user runs `gh team security prs octo/platform`
- **THEN** stdout rows appear in this order: `octo/api#23`, `octo/api#17`, `octo/web#4`

### Requirement: Header line for security PR listing
When `--header` is set and default (TSV) output mode is in effect, the system SHALL emit exactly one tab-separated header line `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl` before any data rows. The header line SHALL be emitted even when zero pull requests match.

#### Scenario: Header line precedes data rows
- **GIVEN** team `octo/platform` owns `octo/api` with one matching open PR
- **WHEN** the user runs `gh team security prs octo/platform --header`
- **THEN** stdout's first line is exactly `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl`
- **AND** the matching pull request's data row follows

#### Scenario: Header still emits on empty result
- **GIVEN** team `octo/empty` owns no repositories with matching pull requests
- **WHEN** the user runs `gh team security prs octo/empty --header`
- **THEN** stdout is exactly `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl\n`
- **AND** the exit status is 0

### Requirement: Invalid title regex is rejected before any repository call
When `--title` is set to a value that fails `regexp.Compile`, the system SHALL exit non-zero before issuing any pull request request and SHALL print an error to stderr that names the offending pattern and the underlying compile error. Stdout SHALL be empty.

#### Scenario: Bad title regex fails fast
- **WHEN** the user runs `gh team security prs octo/platform --title '['`
- **THEN** the command exits with a non-zero status
- **AND** stderr names the invalid pattern `[` and the regex compile error
- **AND** stdout is empty
- **AND** no pull request request is issued

### Requirement: Per-repository failures follow the partial-failure pattern
When the system cannot list pull requests for a particular owned repository (HTTP 403, 404, or other non-fatal error), the system SHALL continue processing the remaining repositories, SHALL print a warning to stderr naming the affected repository and the failure category, and SHALL exit with status `1` after rendering any successful results. Successful rows SHALL still be printed in the documented order.

#### Scenario: Partial failure still renders successful rows
- **GIVEN** team `octo/platform` owns `octo/api` and `octo/web`
- **AND** pull request listing succeeds for `octo/api` with one matching row
- **AND** pull request listing fails for `octo/web` with HTTP 403
- **WHEN** the user runs `gh team security prs octo/platform --header`
- **THEN** stdout begins with the `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl` header line followed by the matching row from `octo/api`
- **AND** stderr includes a warning naming `octo/web` and the access-denied category
- **AND** the command exits with status `1`
