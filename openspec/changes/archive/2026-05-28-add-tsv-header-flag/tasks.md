## 1. Flag plumbing

- [x] 1.1 Add a `--header` boolean field to the shared `outputFlags` struct in `cmd/output.go`
- [x] 1.2 Attach `--header` to `gh team repo list`, `gh team security summary`, and `gh team security alerts` via the existing `attach` helper
- [x] 1.3 Reject `--header` when `--json` or `--template` is also set, with a non-zero exit and an error message that names both flags

## 2. Header emission

- [x] 2.1 Extend the shared `outputPlan` render path so the default-mode branch prepends a single tab-separated header line when `--header` is set
- [x] 2.2 Header content is fixed per command — define one canonical tab-joined string per command alongside its default renderer
- [x] 2.3 For `gh team repo list`, switch the default-mode row format to the four-column TSV (`owner\tname\tfull_name\tarchived`, with `archived` rendered as `true`/`false`) when `--header` is set; keep the existing single-column `<org>/<repo>` rows when `--header` is not set
- [x] 2.4 Leave `security summary` and `security alerts` data rows unchanged — they already emit the columns named in the header
- [x] 2.5 Emit the header line even when the row set is empty
- [x] 2.6 Keep the no-flag default-mode output byte-for-byte unchanged for every supported command

## 3. Docs

- [x] 3.1 Update README to document `--header`, the per-command header strings, and the "default-mode only" constraint
- [x] 3.2 Update command long help text to mention `--header`
- [x] 3.3 Add a README example showing TSV-with-header piped or imported into a spreadsheet

## 4. Verification

- [x] 4.1 Add a unit test that confirms `--header` off (default) keeps the v0.3.0 default-mode bytes for every supported command
- [x] 4.2 Add unit tests that assert the exact header string and column order per command, including the four-column row shape for `repo list --header`
- [x] 4.3 Add tests for `--header` with an empty result set (header still emits)
- [x] 4.4 Add tests for `--header --json` and `--header --template` rejection with exit status and stderr wording
- [x] 4.5 Add a security-command test that verifies `--header` does not interfere with stderr warnings on partial-failure runs
