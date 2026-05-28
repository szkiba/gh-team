## Why

`gh team` is already script-first, but current output is fixed TSV/plain-text per command. That works for shell pipelines, yet it forces users into post-processing (`cut`, `awk`, `sed`, `jq` adapters) for even small layout changes.

A shared output contract for data-emitting commands would make automation safer (`--json`) and interactive use more flexible (`--template`) while preserving existing default output compatibility.

## What Changes

- Add shared output flags for data-emitting subcommands:
  - `--json` for structured array output.
  - `--template <go-template>` for one-line-per-item custom rendering.
- Keep current default stdout format unchanged when neither flag is provided.
- Make `--json` and `--template` mutually exclusive.
- Apply these flags to:
  - `gh team repo list`
  - `gh team security summary`
  - `gh team security alerts`
- Keep warnings and operational diagnostics on stderr exactly as today.
- Keep side-effect command semantics unchanged:
  - `gh team repo clone` does not add output-format flags.

## Other Output-Flag Ideas Considered

- `--output <path>`: deferred (can be done by shell redirection; adds file-overwrite semantics and permissions complexity).
- `--format tsv|json|template`: deferred in favor of clearer explicit flags for v1.
- `--no-warnings`: rejected for now (would hide partial-failure information important for security commands).
- `--color` / table rendering flags: deferred (project favors deterministic pipe-friendly output over terminal decoration).

## Capabilities

### Modified Capabilities

- `team-cli`: add shared output-mode contract and flag-validation behavior.
- `team-repo`: extend `repo list` with JSON and template output modes.
- `team-security`: extend `security summary` and `security alerts` with JSON and template output modes.

## Impact

- Affected code: command flag plumbing in `cmd/`, output rendering helpers, command tests, and README/help text.
- User-facing behavior: existing default output unchanged; optional structured and templated output for report-style commands.
- Compatibility: output modes become a documented contract; field names used by JSON/template contexts are stable once released.
