package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/spf13/cobra"
)

// outputFlags carries the raw `--json` / `--template` values for any
// data-emitting subcommand. Resolve() turns them into a typed outputPlan
// and validates mutual exclusion + template parsing.
//
// The template engine is configured with `missingkey=error` so that a
// reference to a field that does not exist on the row context — for
// example a typo like `{{.full_nam}}` — produces a template execution
// error rather than rendering `<no value>` into stdout. Lower-case JSON
// field names force the row context to be map-backed, and stock
// text/template would silently substitute `<no value>` for unknown map
// keys without this option.
type outputFlags struct {
	json     bool
	template string
}

// attach registers the shared output flags on a Cobra command. Kept in
// one place so help text and defaults stay aligned across commands.
func (o *outputFlags) attach(c *cobra.Command) {
	c.Flags().BoolVar(&o.json, "json", false,
		"render output as a JSON array instead of the default line format")
	c.Flags().StringVar(&o.template, "template", "",
		"render each item using a Go text/template; produces exactly one line per item")
}

// outputMode is the resolved decision for how a command will write its
// stdout dataset. Default mode preserves the command's existing
// line-oriented output.
type outputMode int

const (
	outputDefault outputMode = iota
	outputJSON
	outputTemplate
)

// outputPlan is the immutable post-validation rendering decision. Commands
// build their rows, then hand them to plan.render along with a fallback
// default renderer.
type outputPlan struct {
	mode outputMode
	tmpl *template.Template
}

// resolve validates flag combinations and pre-parses the template (when
// set) so a malformed template fails before any GitHub API call is
// issued. Mutual exclusion of `--json` and `--template` is enforced here
// per the team-cli spec.
func (o *outputFlags) resolve() (outputPlan, error) {
	if o.json && o.template != "" {
		return outputPlan{}, fmt.Errorf("--json and --template cannot be combined; pick one output mode")
	}
	if o.json {
		return outputPlan{mode: outputJSON}, nil
	}
	if o.template != "" {
		t, err := template.New("output").Option("missingkey=error").Parse(o.template)
		if err != nil {
			return outputPlan{}, fmt.Errorf("invalid --template: %w", err)
		}
		return outputPlan{mode: outputTemplate, tmpl: t}, nil
	}
	return outputPlan{mode: outputDefault}, nil
}

// defaultRenderer writes one default-mode line for a single row. Each
// command supplies its own so the existing byte-compatible default
// behavior is preserved verbatim — the shared helper never touches the
// default path beyond iterating.
type defaultRenderer func(out io.Writer, row map[string]any) error

// render walks the row set in the order the caller supplied (which is
// already sorted to match the command's default ordering) and writes the
// chosen output mode to `out`. Template and JSON modes both honor that
// caller-supplied order, so template mode preserves the same sort as
// default mode for the same command.
func (p outputPlan) render(out io.Writer, rows []map[string]any, def defaultRenderer) error {
	switch p.mode {
	case outputJSON:
		return p.renderJSON(out, rows)
	case outputTemplate:
		return p.renderTemplate(out, rows)
	default:
		for _, r := range rows {
			if err := def(out, r); err != nil {
				return err
			}
		}
		return nil
	}
}

// renderJSON emits exactly one JSON array followed by a trailing newline
// per the design contract. An empty result still emits `[]\n` so callers
// piping into `jq` always receive parseable input.
func (p outputPlan) renderJSON(out io.Writer, rows []map[string]any) error {
	if rows == nil {
		rows = []map[string]any{}
	}
	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(rows); err != nil {
		return fmt.Errorf("encode JSON: %w", err)
	}
	return nil
}

// renderTemplate executes the template for each row and enforces the
// "exactly one line per item" contract: any newline that is not the
// final byte of the rendered string aborts the run with an actionable
// error. This keeps templates like `{{printf "%s\n%s" .a .b}}` from
// silently emitting two lines per input item, which would break the
// pipe-friendly stdout contract the change line promises.
func (p outputPlan) renderTemplate(out io.Writer, rows []map[string]any) error {
	var buf strings.Builder
	for i, r := range rows {
		buf.Reset()
		if err := p.tmpl.Execute(&buf, r); err != nil {
			return fmt.Errorf("template execution failed for item %d: %w", i, err)
		}
		s := buf.String()
		if idx := strings.IndexByte(s, '\n'); idx >= 0 && idx != len(s)-1 {
			return fmt.Errorf("template produced multiple lines for item %d; --template must render exactly one line per item", i)
		}
		if !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		if _, err := io.WriteString(out, s); err != nil {
			return err
		}
	}
	return nil
}
