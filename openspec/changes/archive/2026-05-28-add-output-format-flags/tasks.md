## 1. Output contract

- [x] 1.1 Define shared output-mode options for data-emitting commands (`default`, `json`, `template`)
- [x] 1.2 Enforce mutual exclusion of `--json` and `--template`
- [x] 1.3 Define and document stable field names for template context and JSON objects

## 2. Command wiring

- [x] 2.1 Add `--json` and `--template` to `gh team repo list`
- [x] 2.2 Add `--json` and `--template` to `gh team security summary`
- [x] 2.3 Add `--json` and `--template` to `gh team security alerts`
- [x] 2.4 Keep `gh team repo clone` behavior unchanged (no output-format flags)

## 3. Rendering

- [x] 3.1 Keep default stdout output byte-compatible with current behavior when no new flag is set
- [x] 3.2 Implement JSON array output for each supported command
- [x] 3.3 Implement one-line-per-item Go-template output for each supported command, using a `map[string]any` context with `template.Option("missingkey=error")` and rejecting renderings that contain embedded newlines
- [x] 3.4 Keep warning/error streams on stderr with existing partial-failure semantics

## 4. Docs

- [x] 4.1 Update command help text and README examples for `--json` and `--template`
- [x] 4.2 Document field names and compatibility expectations for output schemas
- [x] 4.3 Document deferred output-flag ideas and rationale (no `--no-warnings`, no output file flag in this change)

## 5. Verification

- [x] 5.1 Add tests for default output compatibility (no flags)
- [x] 5.2 Add tests for `--json` output shape and deterministic ordering
- [x] 5.3 Add tests for `--template` rendering, newline handling, and ordering — template output must match default-mode order for each supported command
- [x] 5.4 Add tests for invalid template parse/execute errors, including unknown-field rejection (`missingkey=error`) and embedded-newline rejection in rendered items
- [x] 5.5 Add tests for rejecting `--json` + `--template` together
- [x] 5.6 Add tests ensuring security warnings still appear on stderr and incomplete runs still exit non-zero
