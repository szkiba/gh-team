## 1. PR collection plumbing

- [x] 1.1 Add a `PullRequest` struct in `internal/security` (or a sibling package) with `Repo`, `Number`, `State`, `Title`, `Author`, `Updated`, `URL` fields and JSON tags matching the field-name contract.
- [x] 1.2 Implement a per-repository pull request lister that calls `GET /repos/{owner}/{name}/pulls?state=open&per_page=100`, follows `Link` pagination, and decodes the minimum fields required.
- [x] 1.3 Reuse the existing security collector's concurrency / worker model so PR fetches respect the same bounded fanout.
- [x] 1.4 Surface per-repository failures as warnings on a shared error channel matching the `security alerts` partial-failure pattern (continue, warn, exit `1`).
- [x] 1.5 Add unit tests with a fake HTTP client covering: success, empty result, pagination across multiple pages, HTTP 403 partial failure, and non-200 fatal errors.

## 2. Title and label matching

- [x] 2.1 Implement the default title regex `(?i)^\[security\]|^security:|\[security\]$` and the default label `security`.
- [x] 2.2 Implement a matcher that returns true when the title regex matches OR any label exactly equals one of the configured labels.
- [x] 2.3 Compile the user-supplied `--title` regex once before fanout and exit non-zero with the named pattern and underlying compile error on failure.
- [x] 2.4 Unit-test the matcher against: title match only, label match only, both, neither, case-insensitive title match, repeated `--label` values, and override-replaces-default semantics.

## 3. Cobra subcommand wiring

- [x] 3.1 Add `cmd/security_prs.go` registering `prs` under the existing `security` cobra group.
- [x] 3.2 Declare the command's flags: `--title <regex>`, `--label <l>` (string slice, repeatable), plus the inherited root flags.
- [x] 3.3 Wire the v0.4.0 output flags (`--header`, `--json`, `--template`) through the shared output package; reject conflicting combinations identically to `security alerts`.
- [x] 3.4 Hand off resolution and PR collection to the new internal package, then drive output through the shared writer with the seven-field contract.

## 4. Default-mode formatting and sanitization

- [x] 4.1 Render `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl` in default and `--header` modes, with tabs and newlines in `title` replaced by single spaces before write.
- [x] 4.2 Render `updated` as ISO-8601 UTC with `Z` suffix.
- [x] 4.3 Render `--json` rows preserving the original title verbatim; `.number` SHALL be a JSON integer.
- [x] 4.4 Apply the `repo asc, number desc` sort to the collected rows before writing.

## 5. Tests at the command layer

- [x] 5.1 Golden-output test: `gh team security prs <team>` over a fixture with title-matching, label-matching, and non-matching PRs across two repos verifies row content and ordering.
- [x] 5.2 Header / JSON / template tests mirroring the existing `cmd/security_alerts_output_test.go` shape, including the `--json` title-preserves-tabs assertion.
- [x] 5.3 Override tests: `--title` alone replaces the title default while the label default still applies; `--label` repeated replaces the label default; both set replaces both.
- [x] 5.4 Invalid `--title` regex test: command exits non-zero, stdout empty, stderr names the pattern and compile error, no HTTP calls made.
- [x] 5.5 Partial-failure test: one repo returns 403, another returns rows; stdout contains the successful rows, stderr names the failing repo, exit status is `1`.
- [x] 5.6 Output-flag conflict tests for `--header --json`, `--header --template`, `--json --template`.

## 6. Documentation

- [x] 6.1 Update `README.md` to document `gh team security prs`, the default signals, override flags, the seven-column field-name contract row, the title-sanitization rule, the v1 state-open-only limitation, and the partial-failure exit behavior.
- [x] 6.2 Add `gh team security prs octo/platform`, `--header`, `--json`, and `--template` examples alongside the existing security examples.
- [x] 6.3 Note the deferred follow-ups in the README's deferred-ideas section: `--state` flag, author-based defaults, and Dependabot-security cross-linking.
- [x] 6.4 Extend the README's auth / scope guidance to call out that `gh team security prs` against private repositories requires repository-read access on the host `gh` session (classic OAuth `repo`, or fine-grained `Pull requests: read`), in addition to `read:org` for ownership resolution.

## 7. Auth-error remediation

- [x] 7.1 Detect a missing repository-read scope on the `pulls` endpoint distinctly from the existing security-events scope failure, so the surfaced message names the correct remediation.
- [x] 7.2 Emit guidance that recommends `gh auth refresh -s repo` (or widening the equivalent fine-grained `Pull requests: read` permission) and explicitly does NOT recommend `gh auth refresh -s read:org,security_events` for this command family.
- [x] 7.3 Unit-test the auth-error surface: a fake client returning 403 on `pulls` for a private repository while ownership resolution succeeded produces the `repo`-scoped remediation; behavior for `security alerts` is unchanged.

## 8. Validation

- [x] 8.1 Run `go test ./...` and confirm all suites pass.
- [x] 8.2 Run `openspec validate add-team-security-prs --strict` and resolve any reported issues.
- [ ] 8.3 Manually exercise the command against a real team in default, `--header`, `--json`, and `--template` modes and confirm row shape, sort order, and exit codes match the spec.
