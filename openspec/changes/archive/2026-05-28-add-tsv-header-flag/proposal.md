## Why

The default TSV stdout from `gh team repo list`, `gh team security summary`, and `gh team security alerts` is pipe-friendly but unlabeled. Users who paste the output into Excel or Google Sheets, or who hand it off to a colleague, have to remember the column order or look it up in the README. A single optional header line removes that ambiguity at zero cost to existing pipelines, because the flag stays off by default.

## What Changes

- Add a shared `--header` boolean flag to the three data-emitting subcommands:
  - `gh team repo list`
  - `gh team security summary`
  - `gh team security alerts`
- When `--header` is set, prepend a single tab-separated line of field names before the first data row on stdout. The field names SHALL match the public JSON / template field-name contract published in v0.3.0.
- For `gh team repo list`, whose default mode is a single `<org>/<repo>` per line, setting `--header` SHALL also widen each data row to the same four-column TSV named in the header (`owner\tname\tfull_name\tarchived`) so the labeled output is structurally consistent. The no-flag default mode remains single-column.
- Keep the default behavior unchanged when `--header` is not set, so existing scripts and the v0.3.0 byte contract for default mode continue to hold.
- `--header` SHALL be valid only in default (TSV) mode. Combining it with `--json` or `--template` SHALL be rejected with a non-zero exit and an actionable error.
- Apply `--header` to data-emitting commands only. `gh team repo clone` is unaffected.
- Even when the result set is empty, `--header` SHALL still emit the header line so a downstream spreadsheet pre-populates its column names.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `team-cli`: extend the shared output-mode contract so `--header` is documented as a default-mode modifier and its incompatibility with `--json` / `--template` is part of the public flag contract.
- `team-repo`: extend `repo list` with the `--header` flag and its header line for default mode.
- `team-security`: extend `security summary` and `security alerts` with the `--header` flag and the matching default-mode header lines.

## Impact

- Affected code: command flag plumbing in `cmd/`, the shared output helper in `cmd/output.go`, README + help text, and command tests.
- External APIs: none.
- User-facing behavior: default output without `--header` is byte-compatible with v0.3.0. Adding `--header` prepends a single tab-separated line of field names so the output is directly importable into Excel / Google Sheets with the first row treated as the header.
- Compatibility: the header field names re-use the v0.3.0 public field-name contract. Removing or renaming those names remains a separate breaking-change decision.
