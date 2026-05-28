## ADDED Requirements

### Requirement: Data-emitting commands support shared output modes
The system SHALL support shared output-mode flags for data-emitting `gh team` subcommands. In this change, the covered commands are `gh team repo list`, `gh team security summary`, and `gh team security alerts`.

The supported output modes are:
- default mode (no output flag): existing line-oriented stdout format.
- JSON mode (`--json`): structured JSON array output.
- template mode (`--template <go-template>`): exactly one rendered line per output item, with embedded newlines treated as an error.

#### Scenario: Shared output mode on a supported command
- **WHEN** the user runs `gh team security summary octo/platform --json`
- **THEN** stdout is valid JSON representing an array of summary items
- **AND** exit status semantics match the command's existing behavior

#### Scenario: Unsupported command remains unchanged
- **WHEN** the user runs `gh team repo clone octo/platform`
- **THEN** the command behavior is unchanged by output-mode features in this change

### Requirement: Output flags are mutually exclusive
The system SHALL reject using `--json` and `--template` together on the same invocation with a non-zero exit status and an error message explaining that only one output mode may be selected.

#### Scenario: Conflicting output flags
- **WHEN** the user runs `gh team repo list octo/platform --json --template '{{.full_name}}'`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that `--json` and `--template` cannot be combined

### Requirement: Template errors are actionable
When template parsing or execution fails, the system SHALL exit with a non-zero status and print an actionable error to stderr that names the template failure. The template engine SHALL be configured with `missingkey=error` so that a reference to a field that does not exist on the command's template context (for example, a typo such as `{{.full_nam}}`) is reported as an execution error rather than rendered as `<no value>`.

#### Scenario: Invalid template syntax
- **WHEN** the user runs `gh team security alerts octo/platform --template '{{.repo'`
- **THEN** the command exits with a non-zero status
- **AND** stderr reports a template parse error

#### Scenario: Unknown template field is rejected
- **WHEN** the user runs `gh team repo list octo/platform --template '{{.full_nam}}'`
- **THEN** the command exits with a non-zero status
- **AND** stderr reports a template execution error naming the missing key
- **AND** stdout does NOT contain a `<no value>` placeholder

### Requirement: Template output is strictly one line per item
When a template-mode command renders an item, the resulting line SHALL contain no embedded newlines. The renderer SHALL append a single trailing newline when the rendered string does not already end with one. If the rendered string contains a newline that is not the final character, the system SHALL fail the entire command with a non-zero exit and an error message identifying the offending item, instead of writing more than one line of stdout for a single input item.

#### Scenario: Trailing newline normalization
- **GIVEN** team `octo/platform` owns repository `octo/api`
- **WHEN** the user runs `gh team repo list octo/platform --template '{{.full_name}}'`
- **THEN** stdout contains exactly one line per repository, each terminated by a single `\n`

#### Scenario: Embedded newline in rendered item is rejected
- **WHEN** the user runs `gh team repo list octo/platform --template '{{printf "%s\n%s" .owner .name}}'`
- **THEN** the command exits with a non-zero status
- **AND** stderr explains that the template produced more than one line for a single item
- **AND** stdout does not include a multi-line rendering of any item
