## MODIFIED Requirements

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
