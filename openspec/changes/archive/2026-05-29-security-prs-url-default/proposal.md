## Why

`gh team security prs` default mode currently emits a wide 7-column TSV (`repo\tnumber\tstate\ttitle\tauthor\tupdated\turl`). That makes the default output hard to pipe into common follow-up commands (browser open, `xargs gh pr view`, etc.) and is inconsistent with the sibling `gh team repo list` default, which emits a single URL-like identifier per line and only widens to the full row shape when `--header` or `--json` is set. Aligning the two subcommands gives users one mental model: default = "one identifier per line, pipeable", `--header`/`--json` = "full record".

## What Changes

- **BREAKING**: `gh team security prs <team>` default mode (no flag) emits one pull-request URL per line, instead of the current 7-column TSV row. Rows are still sorted by `repo` ascending, then `number` descending; lines are `<url>\n`.
- `--header` continues to emit the 7-column TSV row shape that today is the unflagged default, prefixed by the header line `repo\tnumber\tstate\ttitle\tauthor\tupdated\turl`. The 7-column row contract (sanitization, column order, field names) is unchanged when `--header` is set.
- `--json` and `--template` output are unchanged. Field names (`.repo`, `.number`, `.state`, `.title`, `.author`, `.updated`, `.url`) and the JSON title-verbatim rule still apply.
- Help text and command long-description updated to describe the new default and reference the `--header` flag for the wide TSV shape.

## Capabilities

### New Capabilities
<!-- none -->

### Modified Capabilities
- `team-security`: change the default-mode output contract for `security prs` (single-URL lines) and re-scope the `--header` requirement to also gate the 7-column TSV row shape.
- `team-cli`: update the shared `--header` contract so that `security prs` joins `repo list` as a subcommand whose default mode is one column and whose `--header` mode widens each row, and update the "`security prs` honors the shared output-flag contract" requirement so it references the new URL-only default instead of the seven-column TSV default.

## Impact

- Affected code: [cmd/security_prs.go](cmd/security_prs.go) (`renderPullDefault`, default row formatter, command long description), [cmd/output.go](cmd/output.go) (`--header` flag help text at [cmd/output.go:37-38](cmd/output.go#L37-L38) still claims `--header` only "prepends a header line"), [cmd/security_prs_output_test.go](cmd/security_prs_output_test.go), [cmd/security_partial_output_test.go](cmd/security_partial_output_test.go) — any test that asserts on default-mode multi-column rows.
- BREAKING change to existing v0.x users of `gh team security prs` who parse default-mode TSV columns. The migration is not a byte-for-byte drop-in: switching to `--header` recovers the seven-column TSV row shape but **adds a leading header line** that consumers must either skip (e.g. `tail -n +2`) or accept. Scripts that read column 1 of line 1 must be updated. Mitigation: CHANGELOG and README both spell out the header-line skip step.
- No GitHub API surface changes. No new flags. No dependencies added.
- Docs to update: README "Output flags" section at [README.md:56-62](README.md#L56-L62) — the `--header` bullet currently names only `repo list` as widening under `--header`; it must also name `security prs`. CHANGELOG entry for the next release marked **BREAKING** with the header-line skip note.
