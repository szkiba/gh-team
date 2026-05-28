## Context

`gh team` ships three data-emitting commands that print TSV by default and accept `--json` or `--template` for richer modes. The TSV is pipe-friendly but unlabeled, so importing into Excel or Google Sheets requires the user to remember the column order or look it up in the README.

This change adds an opt-in `--header` flag that prepends a single tab-separated line of field names ahead of the existing data rows, without changing any other behavior. The field names re-use the v0.3.0 public JSON / template field-name contract.

## Goals / Non-Goals

**Goals:**
- Make TSV output directly importable into Excel / Google Sheets with the first row treated as the spreadsheet header.
- Keep default-mode output byte-compatible with v0.3.0 when `--header` is not set.
- Re-use the existing field-name contract; do not introduce a second naming scheme.
- Treat `--header` as orthogonal to all other flags except `--json` and `--template`, which are explicitly rejected.

**Non-Goals:**
- Apply a header to `--json` (it would not be valid JSON) or `--template` (the user already controls layout).
- Configurable header text or formatting (`--header-prefix`, `--header-uppercase`, etc.).
- Per-command header customization. Every command's header is determined by its v0.3.0 field-name contract.
- Adding a header to `gh team repo clone`; clone is a side-effect command and does not emit a dataset.

## Decisions

### 0. `repo list --header` widens data rows to the four-column TSV shape

`gh team repo list` default output (no flag) is a single `<org>/<repo>` per line. The v0.3.0 JSON / template contract for the same command exposes four fields: `owner`, `name`, `full_name`, `archived`. A four-column header above one-column data rows is structurally inconsistent and defeats the spreadsheet-import goal.

When `--header` is set on `repo list`, the data rows that follow SHALL also be tab-separated and SHALL carry exactly the four fields named in the header line, in the same order:
`<owner>\t<name>\t<full_name>\t<archived>`.

The archived flag is rendered as the lower-case strings `true` or `false` so the resulting cell type in Sheets / Excel matches the JSON boolean. Items remain sorted alphabetically by `full_name` to match every other mode.

When `--header` is NOT set, `repo list` keeps emitting single-column `<org>/<repo>` lines exactly as v0.3.0 promised.

For `security summary` and `security alerts`, the default-mode rows are already multi-column TSV that matches each command's JSON contract; `--header` simply prepends the header line and leaves rows unchanged.

Why:
- The whole point of `--header` is "labeled TSV that imports cleanly into a spreadsheet." A header that does not describe the rows below it fails that goal.
- Users who opt into `--header` are explicitly choosing the labeled spreadsheet shape; they are not in the no-flag byte-compatibility path.
- Re-using the existing JSON field names keeps the contract single-sourced.

Alternative considered:
- Keep `repo list --header` data rows as one column and emit a one-field header `full_name`.
  Rejected. A one-column TSV does not need a header to be importable, and the user would still have to use `--json` to see `owner`, `name`, or `archived` — defeating the value of `--header`.

### 1. `--header` is a default-mode modifier, not a fourth output mode

`--json` and `--template` are mutually exclusive modes. `--header` is not a mode — it is a modifier that only applies when default (TSV) mode is in effect. Combining `--header` with `--json` or `--template` is rejected at flag-resolve time with a non-zero exit.

Why:
- Adding a header to JSON output would make it invalid JSON, defeating the purpose of `--json`.
- Adding a header to `--template` would force a header into rendered output the user explicitly designed.
- The "modifier" framing also lets future modifiers (for example a `--no-trailing-newline` or `--separator=`) sit on the same shelf without overloading the mode taxonomy.

Alternative considered:
- Treat `--header` as a fourth output mode (`tsv-with-header`).
  Rejected because the data rows are still TSV; only the header is new. A mode for "TSV plus one header line" overstates the structural change.

### 2. Field names are the v0.3.0 contract verbatim

The header line is a tab-separated list of the same lower-case field names used by `--json` and `--template`:
- `repo list` → `owner\tname\tfull_name\tarchived`
- `security summary` → `repo\tfamily\tcount`
- `security alerts` → `family\trepo\tkey\tseverity\turl`

Why:
- Users who already know the JSON / template field names will read the TSV header without surprise.
- Spreadsheet column headers double as JSON keys after `Save as CSV → Import → JSON`, which is a common workflow.
- A second naming scheme would create two contracts to evolve together.

Alternative considered:
- Use Title Case ("Owner", "Full Name") or human labels ("Open Alerts").
  Rejected. The TSV header is still pipe-friendly output, not a presentation layer; matching the JSON contract is more valuable than cosmetic capitalization.

### 3. Header still emits when the result set is empty

`--header` always emits the header line, even for zero data rows.

Why:
- Pre-populating spreadsheet column headers is one of the primary motivations.
- An empty header on an empty result would force the user to remember the column order anyway.

Alternative considered:
- Suppress header when there are zero rows.
  Rejected. Output predictability matters more than saving one line of stdout in an already-empty result.

### 4. Default-mode byte contract still holds without the flag

When `--header` is not set, stdout for every data-emitting command stays byte-for-byte identical to v0.3.0. That is the contract that protects existing scripts. Setting `--header` is an explicit opt-in to the labeled-spreadsheet shape — for `repo list` this also widens data rows per Decision 0; for security commands the data rows are already multi-column TSV.

Why:
- v0.3.0 promised "Default output remains unchanged" and that promise has to keep holding for existing no-flag invocations.
- A separate flag flips into the new behavior, so no v0.3.0 invocation can accidentally see a header or a wider row shape.

## Risks / Trade-offs

- [Header line is visually indistinguishable from a repo named "owner" or a family named "repo"] → Document that automation should consume `--json` instead of `--header`. The header is meant for human-readable spreadsheet import, not script parsing.
- [Tab in a field name would break the header] → Not currently a risk: all v0.3.0 field names are simple ASCII with no tabs or whitespace.
- [Adding `--header` to `--json` later is a breaking change] → Decision 1 explicitly rejects that combination, so the door is closed unless a future change reopens it.

## Migration Plan

No data migration required. Rollout is a normal additive extension update. Rollback removes the new flag and helper code; no spec or data state to undo.

## Open Questions

- None for v1. Future expansions (header customization, additional modifiers) are deferred to separate changes.
