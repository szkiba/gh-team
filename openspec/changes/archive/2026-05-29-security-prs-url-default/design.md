## Context

`gh team security prs` was added in v0.3.x and adopted the 7-column TSV row as default output mode. Around the same time, `gh team repo list` followed a different convention: default mode is a single identifier per line, and `--header` widens it to a multi-column TSV. The asymmetry surfaced when users tried to compose `security prs` output with `xargs gh pr view`, `gh pr checkout`, or shell pipelines that expect a single token per line.

The relevant code is concentrated in [cmd/security_prs.go](cmd/security_prs.go), specifically the `renderPullDefault` step that produces the 7-column TSV row. The existing default row formatter at [cmd/security_prs.go:148-163](cmd/security_prs.go#L148-L163) does the column join and `sanitizeTitleCell` step. `--json` and `--template` output paths are independent and need no changes.

This change is BREAKING for any consumer that parses the current default-mode TSV columns. We intentionally accept the break because (a) the project is pre-1.0, (b) `--header` is already documented in the same release that shipped `security prs`, and (c) the migration path is small: adding `--header` recovers the seven-column TSV row shape, although consumers must also skip the new leading header line (e.g. `tail -n +2`) — it is not a byte-for-byte replacement.

The shared output-flag contract lives in [openspec/specs/team-cli/spec.md](openspec/specs/team-cli/spec.md). Two requirements there currently describe `security prs` default mode as the seven-column TSV and describe `--header` as "leaves data rows unchanged for `security prs`": both must be amended in this change so the spec set stays internally consistent.

## Goals / Non-Goals

**Goals:**
- Default output of `gh team security prs <team>` is one PR URL per line, `<url>\n`, sorted by `repo` asc then `number` desc.
- `--header` continues to produce the full 7-column TSV row shape, prefixed by the existing header line.
- `--json` and `--template` behavior — including field names and the JSON title-verbatim rule — stay byte-for-byte identical.
- Help / long-description text reflects the new default and points at `--header` for the wide row.

**Non-Goals:**
- Adding a deprecation shim or a `--legacy-default` flag. Migration is `--header`.
- Changing the default sort key set or the title sanitization rule.
- Changing `--json` field names, types, or order.
- Changing the behavior of `repo list`; this change only aligns `security prs` with it.

## Decisions

**Decision: Default mode emits one URL per line, not `<repo>#<number>` or another identifier.**
Rationale: URL is the most directly actionable token — `xargs gh pr view <url>` and `xargs -I{} open {}` both work, and a URL uniquely identifies the PR across orgs and repos. `<repo>#<number>` would require parsing on the consumer side. The `repo list` precedent emits `<owner>/<name>`, but for PRs there is no equally short canonical identifier, so we use the URL.
Alternatives considered: `<repo>#<number>` (rejected: not directly usable with `gh pr view`); `<number>` alone (rejected: ambiguous across repos).

**Decision: `--header` widens to today's exact 7-column TSV row.**
Rationale: Mirrors `repo list --header`, which widens its single-column default to the four-column TSV. Users who currently rely on TSV recover the row shape by adding one flag — though their script must also skip the new leading header line, since `--header` always emits the header (per the existing shared `team-cli` contract that mandates the header line even on empty result sets).
Alternatives considered: introducing a separate `--wide` flag with no header line (rejected: would diverge from the established `repo list --header` pattern and add a second flag for what is effectively the same "wide TSV" mode); leaving the wide row as the default and adding `--short` (rejected: still leaves `security prs` inconsistent with `repo list`).

**Decision: Sorting and `sanitizeTitleCell` behavior are unchanged.**
Rationale: Sort order is part of the cross-mode contract (default, `--header`, `--json`, `--template` all sort identically). URL-only default mode keeps the same sort. `sanitizeTitleCell` only affects the title column, which doesn't appear in URL-only default mode anyway — it still applies under `--header`.

**Decision: This is a single-release breaking change, not a deprecation window.**
Rationale: `security prs` shipped in the same minor series; no consumers are pinned to the old default for long. The migration path (`--header`) is a one-flag change. Documenting in CHANGELOG is sufficient.

## Risks / Trade-offs

- [Existing TSV consumers break silently — pipelines that read columns 1..7 now read only a URL.] → Mitigation: CHANGELOG entry calls out the new default, the `--header` recovery path, and the need to skip the leading header line (e.g. `tail -n +2`). README example for `security prs` updated to lead with default mode plus a `--header` example with the header-skip shown.
- [Users who actually want the 7-column row may not discover `--header`.] → Mitigation: command long-description prominently mentions `--header`. `--help` output groups it under "Output". Same pattern as `repo list`.
- [Future flag additions could re-tempt us into a different "wide" default.] → Mitigation: capture the pattern in spec ("default = one identifier per line, --header = full row") so any new `gh team <noun> list/prs` subcommand inherits it.

## Migration Plan

1. Land code + tests in one commit.
2. CHANGELOG: "**BREAKING:** `gh team security prs` default output is now one PR URL per line. Use `--header` for the previous 7-column TSV shape; note that `--header` always emits a leading header row, so scripts that previously consumed the unflagged TSV must skip line 1 (e.g. `tail -n +2`)."
3. README example block for `security prs` shows both forms, including the header-skip pipeline. README "Output flags" section updated so the `--header` bullet names `security prs` (alongside `repo list`) as a subcommand widened by `--header`.
4. Rollback: revert the commit. The `--header`, `--json`, `--template` contracts are unchanged so a revert affects only the default-mode formatter and the docs.
