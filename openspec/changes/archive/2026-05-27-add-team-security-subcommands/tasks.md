## 1. Command surface

- [x] 1.1 Add a `security` command group under the existing `gh team` root
- [x] 1.2 Add `security summary <org/team-slug>` and `security alerts <org/team-slug>` subcommands
- [x] 1.3 Add and validate `--kind=dependabot|code-scanning|all` for security subcommands

## 2. Security collection

- [x] 2.1 Implement a shared security collector that resolves repositories through the existing ownership resolver
- [x] 2.2 Implement repository-level Dependabot alert collection for open alerts
- [x] 2.3 Implement repository-level code-scanning alert collection for open alerts
- [x] 2.4 Follow pagination for every repository/family pair and filter both APIs to `state=open`
- [x] 2.5 Normalize collected alerts into deterministic summary and detailed output records, including the Dependabot key and code-scanning severity rules from the spec
- [x] 2.6 Process repository/family pairs with bounded concurrency
- [x] 2.7 Treat unavailable alert families as zero results and aggregate hard access failures for final exit status `1`

## 3. Error handling and UX

- [x] 3.1 Extend API error translation to surface security-alert scope guidance
- [x] 3.2 Emit clear per-repository warnings for alert-access failures without aborting the entire run
- [x] 3.3 Update root and subcommand help text to describe the maintainer baseline, the `codeowners` caveat, and the supported alert families

## 4. Docs

- [x] 4.1 Update README usage examples and command reference for `gh team security`

## 5. Verification

- [x] 5.1 Add unit tests for `--kind` validation, the fixed `all` alias, and deterministic output ordering
- [x] 5.2 Add unit tests for partial-failure handling, unavailable-feature handling, and the `security_events` scope error path
- [x] 5.3 Add tests that exercise paginated Dependabot and code-scanning fixtures across multiple pages
- [x] 5.4 Add integration-style tests for Dependabot and code-scanning collection over resolved repositories
- [x] 5.5 Add tests or benchmarks that verify bounded-concurrency collection preserves deterministic output
