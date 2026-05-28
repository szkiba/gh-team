## Context

The `security` command area already has `summary` and `alerts`, both built on a per-repository collector that runs concurrently against the resolved team-owned set and surfaces partial failures as stderr warnings plus exit status `1`. The v0.4.0 output flags (`--header`, `--json`, `--template`) are wired through a shared output package in `cmd/output.go`. The new `prs` subcommand re-uses both layers without inventing parallel infrastructure.

The user-facing intent is "show me what security work is currently in flight for this team". That intent splits into two reliable signals — title pattern and label — and a long tail of weaker signals (author = Dependabot bot, body mentions CVE / GHSA). Earlier exploration established that author-based defaults produce false positives because Dependabot opens routine version-bump PRs as well as security ones; signal selection therefore favors title and label over author.

## Goals / Non-Goals

**Goals:**
- Surface open PRs across the team's owned repositories that match a security signal.
- Default to a zero-config view that catches the common `[security]` title convention and an explicit `security` label without forcing flags.
- Allow each default to be replaced individually via `--title <regex>` and `--label <l>`.
- Reuse the existing concurrency, partial-failure, and output-flag plumbing so behavior matches `security alerts`.
- Honor the v0.4.0 field-name contract: column names are identical in default, `--header`, `--json`, and `--template` modes.

**Non-Goals:**
- `--state` flag — v1 is open-only.
- Combiner flag (`--any` / `--all`) — signals are always OR-combined.
- Author-based defaults or a `--author` flag — the noise-to-signal of Dependabot's mixed traffic makes a default unsafe; deferred to a follow-up that can join PRs against Dependabot alerts.
- Mutation (close, label, merge, comment) — read-only.
- `secret-scanning` family — separate change.
- Alert ↔ PR cross-linking — separate change.

## Decisions

### 1. New subcommand, not a new flag on `security alerts`

`security prs` is a sibling of `summary` and `alerts`.

Why:
- The data shape is fundamentally different (PR rows vs. alert rows).
- The default filter semantics (title + label OR-match) have no analog in `alerts`, which is family-based.
- Triadic cohesion (`summary` = counts, `alerts` = findings, `prs` = remediation) makes the area easy to learn.

Alternative considered:
- Add a `--prs` flag to `security alerts`. Rejected because it conflates two different result shapes and bloats the alerts collector with unrelated logic.

### 2. Signals are OR-combined and the combiner is not exposed in v1

A PR matches if its title matches the title regex OR if at least one of its labels exactly equals the configured label string.

Why:
- The triage use case ("find PRs that look security-related") is naturally inclusive.
- An exclusive (AND) combiner is a niche need — users wanting strict intersection can post-filter with `awk` or open a follow-up if it becomes a real ask.
- Two extra flags (`--any` / `--all`) for one niche outcome would widen the surface without earning its keep.

Alternative considered:
- Ship `--any` / `--all` from day one. Rejected as premature flexibility.

### 3. Title default regex covers prefix, suffix, and `security:` style

Default: `(?i)^\[security\]|^security:|\[security\]$`.

Why:
- Three common conventions in practice: `[security] fix X`, `security: fix X`, `fix X [security]`.
- Case-insensitive flag handles `[Security]` / `[SECURITY]`.
- Limited to anchored positions so a PR titled `address insecurity` does not match.

Alternative considered:
- A free-text contains match. Rejected — would match `insecurity`, `unsecured`, etc.

### 4. Label default is exact, single value

Default label: `security`. Match is exact (not prefix, not substring).

Why:
- Most teams that label use the literal `security` tag.
- Exact match matches the existing `--kind` parser style in `security`, where values are strict enums.
- A glob or prefix matcher would surprise users when `security/critical` matches but `critical/security` does not.

Alternative considered:
- Substring or glob match. Rejected — surprise is worse than narrow.

### 5. Overrides replace, not merge

If `--title` is passed, the title default is dropped. If `--label` is passed, the label default is dropped. Defaults still apply for any signal whose flag is absent.

Why:
- Predictable: `--title <regex>` should mean "match exactly this title pattern", not "match either my regex or the default".
- Repeatable `--label` covers the "match any of these labels" need without a combiner flag.

Alternative considered:
- Additive overrides (defaults always retained). Rejected — robs users of the ability to narrow the scope.

### 6. Sort: repo asc, then number desc

Primary sort is `repo` ascending so all PRs from one repository group together (matches `security alerts`). Secondary sort is PR `number` descending so newer PRs surface first within a repo.

Why:
- Repo grouping mirrors `security alerts`.
- Within a repo, "newest first" is the more useful ordering for triage — recently opened PRs are the ones likely needing attention.
- Deterministic, no ties because PR numbers are unique within a repository.

Alternative considered:
- Sort by `updated` descending. Rejected — globally interleaves repos, harder to scan, and `updated` is mutable (bot pushes shuffle the list run-to-run).

### 7. Title sanitization for default and `--header` modes only

Tabs in a PR title are replaced with a single space; newlines likewise. `--json` preserves the original title byte-for-byte. `--template` runs against the original title; the existing `--template` "no embedded newlines" guardrail still applies and rejects template output containing `\n`.

Why:
- TSV breaks if titles contain tabs.
- Single-line-per-item guarantee breaks if titles contain newlines.
- JSON has no such constraint and consumers can render however they like.

Alternative considered:
- Reject PRs whose titles contain tabs / newlines outright. Rejected — silently drops real findings.

### 8. Per-repository REST fanout, not GraphQL search

Calls `GET /repos/{owner}/{name}/pulls?state=open&per_page=100` per repository, paginated. Concurrency reuses the existing security collector's worker model.

Why:
- The existing ownership resolver + collector already match this shape.
- GraphQL search has a 1000-result cap and a much smaller rate-limit pool (30/min) that would force fallbacks.
- Per-repo fetch is cheaper to reason about for permission failures (one repo's denial does not cascade).

Alternative considered:
- GraphQL search. Rejected for the reasons above; can be reintroduced as `--via=search` later if rate-limit pressure shows up.

### 9. Output-flag contract is inherited verbatim

`--header`, `--json`, `--template` work exactly as in v0.4.0 for the other data subcommands. `--header` is rejected with `--json` or `--template`. `--json` and `--template` remain mutually exclusive. The header line and JSON / template field names use the same column vocabulary.

Why:
- Users learn one contract for all data-emitting subcommands.
- The output package already implements it; the new command just declares its field list.

### 10. Auth: PR enumeration on private repos requires repository-read access

Listing pull requests via `GET /repos/{owner}/{name}/pulls` is a repository-read API. For private owned repositories that means the host `gh` OAuth session needs the classic `repo` scope (or an equivalent fine-grained `Pull requests: read` permission). The `read:org` scope already required by ownership resolution is necessary but not sufficient. Public repositories list without a repository-read scope.

The shared `team-cli` "actionable auth guidance" requirement is extended so a 403 / scope-missing failure on the `pulls` endpoint surfaces remediation that names this scope (for example `gh auth refresh -s repo`) instead of falling through to the generic security-events guidance, which would mislead the user.

Unlike the alert collector, where scope-missing 403s are treated as run-wide fatals (every alert endpoint is gated by the same scope, so the run cannot make progress), scope-missing 403s on `pulls` are handled per-repository. Public repositories list without `repo` scope, so a mixed run can still produce rows from public repos while emitting one warning per private repository that fails. Each such warning embeds the targeted `gh auth refresh -s repo` remediation inline, and `HardFailures > 0` carries the non-zero exit signal. Treating the failure as fatal would discard public-repo rows the caller is entitled to.

Why:
- The maintainer-baseline persona almost always operates on private repositories; missing this scope is part of the core success path, not an edge case.
- Generic 403 output (or the existing `security_events` remediation) would point the user at the wrong fix.
- Aligning the remediation with the actual endpoint the command calls keeps the contract honest.
- Per-repository handling matches the documented partial-failure pattern and preserves the rows from repositories the caller can read.

Alternatives considered:
- Use the org-level pulls aggregation endpoint. Rejected because, like the org-level alert endpoints, it requires organization-owner or security-manager access, which is stricter than the maintainer baseline.
- Skip private-repo enumeration silently when the scope is missing. Rejected — silent under-reporting on the security triad would be a security smell of its own.
- Mirror the alert collector and treat scope-missing 403 as run-wide fatal. Rejected — for `pulls`, public repos succeed regardless of token scope, so a global cancel would throw away rows the caller is entitled to.

## Risks / Trade-offs

- [Default regex misses a team's local convention] → Document the default loudly; provide `--title` override.
- [`--label security` won't match repos using `security/critical`] → Document the exact-match semantics; users can pass `--label security/critical --label security`.
- [Fanout cost grows with team size and PR backlog] → Bounded by the existing concurrency limit; pagination uses `per_page=100`. No new throttling flag in v1.
- [PR titles with tabs or newlines mangle TSV] → Sanitize in default / header mode; JSON preserves.
- [User-supplied regex is invalid] → `regexp.Compile` runs before fanout; surface the compile error and exit non-zero with no output.
- [Dependabot security PRs not caught by defaults] → Acknowledged in the proposal as a non-goal; users can pass `--label dependencies` plus a custom regex, and a follow-up change can add cross-linking.

## Open Questions

- Whether `updated` should default to UTC or local time. Lean: UTC, ISO-8601 with `Z`, matching the rest of the project's date handling.
- Whether to emit `state` at all in v1 (always `"open"`). Lean: keep the column so the field-name contract is stable when `--state` ships later.
