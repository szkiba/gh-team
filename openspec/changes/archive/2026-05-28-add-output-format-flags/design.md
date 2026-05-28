## Context

This change adds optional output modes for data-emitting commands while preserving the current default stdout contract.

Covered commands:
- `gh team repo list`
- `gh team security summary`
- `gh team security alerts`

Not covered:
- `gh team repo clone` (side-effect command; no dataset-style stdout contract)

## Goals / Non-Goals

Goals:
- Keep existing default output byte-compatible when no new flag is selected.
- Add a structured mode for automation (`--json`).
- Add a flexible line-rendering mode for CLI users and shell scripts (`--template`).
- Keep warning and error behavior unchanged on stderr.

Non-Goals:
- Add a generic file-output flag.
- Add colored/table output mode.
- Change clone command output behavior.

## Decisions

### 1. Output modes are command-level shared behavior, not root-global semantics

The implementation reuses shared helper logic, but flags are attached only to data-emitting subcommands. This avoids ambiguous behavior on side-effect commands and keeps help text honest.

### 2. Flag set and precedence

Supported flags on covered commands:
- `--json`
- `--template <go-template>`

Rules:
- No flag: default mode.
- `--json`: JSON mode.
- `--template`: template mode.
- Both set: validation error, non-zero exit.

### 3. Default mode remains unchanged

When no output mode flag is supplied, each command preserves existing line format, ordering, and newline behavior.

### 4. JSON output contract

Output shape:
- stdout emits exactly one JSON array.
- A trailing newline is appended after the array for shell friendliness.
- Item order is identical to default mode deterministic ordering.

Per-command item fields:
- repo list: `owner`, `name`, `full_name`, `archived`
- security summary: `repo`, `family`, `count`
- security alerts: `family`, `repo`, `key`, `severity`, `url`

### 5. Template output contract

Template engine:
- Go `text/template`.

Item model:
- One template execution per item.
- Template context is a `map[string]any` keyed by the same lower-case names used as JSON object fields for the command, because the public field contract is lower-case (`.full_name`, `.repo`, etc.) and struct-backed `text/template` fields would have to be exported.

Unknown-field handling:
- The renderer SHALL execute every template with `template.Option("missingkey=error")` so that a reference to a non-existent field (a user typo such as `{{.full_nam}}`) produces a template execution error instead of rendering `<no value>`. This is what makes the "actionable template errors" requirement enforceable on map-backed context.

Newline behavior — strictly one line per item:
- After executing the template for an item, the renderer SHALL inspect the rendered string. If it contains any newline byte (`\n`) other than as the final character, the renderer SHALL fail with a non-zero exit and an error naming the offending item. This keeps the "one line per item" promise honest and stops templates like `{{printf "%s\n%s" .a .b}}` from silently breaking shell pipelines.
- If the rendered string ends with `\n`, it is emitted as-is.
- If the rendered string does not end with `\n`, the renderer SHALL append exactly one.
- Empty result sets emit nothing.

Function set for v1:
- Whatever `text/template` exposes by default is the contract — `printf`, `print`, `println`, indexing, basic pipelines, plus the standard comparison/logic helpers (`eq`, `ne`, `lt`, `gt`, `len`, `slice`, `and`, `or`, `not`, etc.). No custom function map is added in v1.
- An earlier draft of this design tried to enumerate a narrower subset, but plain `text/template` always exposes the full built-in surface, so promising a smaller subset would either be untrue or force the implementation to do extra restriction work this change does not plan for.

Rationale:
- Lower-case field names match the JSON contract and are only reachable cheaply via map-backed data, so missing-key safety has to be configured explicitly.
- A strict "one line per item" rule is more useful to shell users than a permissive "one execution per item" rule and is cheap to enforce post-execution.
- Custom helpers can be added later as additive changes if real usage demands it.

### 6. Error handling

Flag validation errors:
- Conflicting `--json` + `--template` returns non-zero with actionable message.

Template parse/execute errors:
- Return non-zero with message identifying parse or execution failure.

Security partial-failure behavior:
- Unchanged from current contract.
- Successful items still appear on stdout (default/json/template).
- Warnings stay on stderr.
- Final exit remains non-zero when hard failures occurred.

### 7. Compatibility policy

Default output:
- Existing default output is stable and unchanged by this change.

JSON and template field names:
- Field names introduced in this change are public contract.
- Additive fields are allowed in future versions.
- Removing or renaming fields requires a separate breaking-change decision.

Ordering:
- Item order is deterministic and consistent across default/json/template modes.

## Implementation Sketch

1. Add a small shared output-mode helper in the command layer:
- parse mode from flags
- validate mode exclusivity
- route per-item rendering to default/json/template emitters

2. Keep command data collection unchanged:
- repo resolution and security collection behavior remains as-is
- only rendering path changes

3. Add explicit row view models for output contract:
- avoid exposing internal structs directly to template/json serialization
- keep schema evolution controlled

4. Update command help and README examples

5. Add tests:
- default output compatibility
- json rendering shape and ordering
- template rendering and newline normalization
- invalid template parse and execution failures
- conflicting output flags
- stderr warnings + non-zero exit semantics for partial security failures

## Risks / Trade-offs

- Template mode can create user mistakes in expressions.
  Mitigation: clear parse/execute errors and stable field names.

- No custom template helpers in v1 may feel limited.
  Mitigation: add helpers later as additive change if needed.

- JSON becomes a contract that users script against.
  Mitigation: explicit compatibility policy in specs and docs.
