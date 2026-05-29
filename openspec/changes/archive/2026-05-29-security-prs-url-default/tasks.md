## 1. Code

- [x] 1.1 In [cmd/security_prs.go](cmd/security_prs.go), update `renderPullDefault` (or its equivalent default-mode formatter at [cmd/security_prs.go:148-163](cmd/security_prs.go#L148-L163)) so that when `--header` is NOT set the formatter emits `<url>\n` and only `<url>\n` per matching PR.
- [x] 1.2 Keep the existing seven-column TSV row formatter (with `sanitizeTitleCell`) but gate it behind `--header`. Header line emission stays as-is.
- [x] 1.3 Verify sort order (`repo` asc, `number` desc) is preserved across both default URL mode and `--header` mode — sort happens before formatting, no change needed.
- [x] 1.4 Update the command long-description / help text (around [cmd/security_prs.go:33-45](cmd/security_prs.go#L33-L45)) to state default-mode output is one URL per line and reference `--header` for the seven-column TSV shape.
- [x] 1.5 Update the shared `--header` flag help text at [cmd/output.go:37-38](cmd/output.go#L37-L38) so it no longer implies that `--header` only "prepends" a header line — for `repo list` and `security prs`, `--header` also widens each data row. Phrasing must stay one short flag-help sentence.

## 2. Tests

- [x] 2.1 Update [cmd/security_prs_output_test.go](cmd/security_prs_output_test.go): the existing default-mode tests that assert on seven-column TSV must be moved under `--header` (or rewritten) and new default-mode tests must assert URL-only `\n`-terminated lines.
- [x] 2.2 Add a default-mode test for sort ordering (two PRs in one repo plus one PR in a second repo → URL lines appear in `repo` asc, `number` desc order).
- [x] 2.3 Add a default-mode partial-failure test asserting that `stdout` is URL-only, `stderr` names the failing repo, and exit status is `1`. Mirror the `--header` partial-failure assertions in [cmd/security_partial_output_test.go](cmd/security_partial_output_test.go).
- [x] 2.4 Confirm `--json` and `--template` tests pass unchanged. If any of them indirectly assert default-mode shape (e.g. cross-mode ordering), update assertions to the new URL-only default.

## 3. Docs / Release notes

- [x] 3.1 Update README section for `gh team security prs` so the lead example shows URL-only default output and a follow-up example shows `--header` producing the seven-column row (with a `tail -n +2` example for consumers that need to skip the header line).
- [x] 3.2 Update the README "Output flags" section at [README.md:56-62](README.md#L56-L62) — specifically the `--header` bullet — so it names `security prs` (alongside `repo list`) as a subcommand whose data rows widen under `--header`.
- [x] 3.3 Add a CHANGELOG entry under the next release, marked **BREAKING**, describing the new default and the `--header` migration path including the header-line-skip caveat.

## 4. Verification

- [x] 4.1 Run `go test ./...` and confirm green.
- [x] 4.2 Run `go vet ./...` and confirm clean.
- [x] 4.3 Build the binary and manually exercise: `gh team security prs <known-team>`, `gh team security prs <known-team> --header`, `gh team security prs <known-team> --json`. Confirm shapes match spec.
- [x] 4.4 Run `openspec validate security-prs-url-default --strict` and confirm pass.
