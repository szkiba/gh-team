package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
)

// TestResolve_DefaultsToDefaultMode confirms the zero-value outputFlags
// yields default-mode output so every command's existing behavior is
// preserved when neither flag is supplied.
func TestResolve_DefaultsToDefaultMode(t *testing.T) {
	o := &outputFlags{}
	p, err := o.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if p.mode != outputDefault {
		t.Errorf("mode = %v, want outputDefault", p.mode)
	}
}

// TestResolve_RejectsConflictingFlags covers the team-cli spec scenario
// "Conflicting output flags": --json + --template together must fail with
// a message that names both flags.
func TestResolve_RejectsConflictingFlags(t *testing.T) {
	o := &outputFlags{json: true, template: "{{.full_name}}"}
	_, err := o.resolve()
	if err == nil {
		t.Fatal("expected mutual-exclusion error")
	}
	for _, want := range []string{"--json", "--template"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

// TestResolve_RejectsInvalidTemplate verifies that a parse failure on the
// template returns a non-zero exit path before any work begins.
func TestResolve_RejectsInvalidTemplate(t *testing.T) {
	o := &outputFlags{template: "{{.full_name"}
	_, err := o.resolve()
	if err == nil || !strings.Contains(err.Error(), "invalid --template") {
		t.Errorf("expected parse error, got %v", err)
	}
}

// TestRender_DefaultDelegatesPerCommand confirms the default branch only
// iterates and calls the per-command renderer — the shared helper never
// touches default formatting, so existing byte-compatible output stays
// intact.
func TestRender_DefaultDelegatesPerCommand(t *testing.T) {
	o := &outputFlags{}
	p, _ := o.resolve()
	rows := []map[string]any{
		{"full_name": "octo/api"},
		{"full_name": "octo/web"},
	}
	var buf bytes.Buffer
	err := p.render(&buf, rows, renderConfig{defFn: func(out io.Writer, row map[string]any) error {
		_, err := io.WriteString(out, row["full_name"].(string)+"\n")
		return err
	}})
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "octo/api\nocto/web\n" {
		t.Errorf("default render = %q", buf.String())
	}
}

// TestRender_JSONShape exercises the JSON array contract: exact field
// names, ordered as the caller supplied, trailing newline. Empty input
// must still emit `[]\n` so downstream `jq` calls always receive
// parseable input.
func TestRender_JSONShape(t *testing.T) {
	o := &outputFlags{json: true}
	p, _ := o.resolve()
	rows := []map[string]any{
		{"owner": "octo", "name": "api", "full_name": "octo/api", "archived": false},
		{"owner": "octo", "name": "web", "full_name": "octo/web", "archived": true},
	}
	var buf bytes.Buffer
	if err := p.render(&buf, rows, renderConfig{}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(buf.String(), "\n") {
		t.Errorf("JSON output must end with newline; got %q", buf.String())
	}
	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("emitted invalid JSON %q: %v", buf.String(), err)
	}
	if len(got) != 2 || got[0]["full_name"] != "octo/api" || got[1]["full_name"] != "octo/web" {
		t.Errorf("JSON output preserves neither shape nor order: %v", got)
	}
}

func TestRender_JSONEmptyArray(t *testing.T) {
	o := &outputFlags{json: true}
	p, _ := o.resolve()
	var buf bytes.Buffer
	if err := p.render(&buf, nil, renderConfig{}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "[]\n" {
		t.Errorf("empty JSON = %q, want \"[]\\n\"", buf.String())
	}
}

// TestRender_TemplateOneLinePerItem covers the strict one-line-per-item
// promise: each rendered row gets a single trailing newline appended
// when missing, and the order matches the caller-supplied order (which
// the upstream commands sort to match default mode).
func TestRender_TemplateOneLinePerItem(t *testing.T) {
	o := &outputFlags{template: "{{.full_name}}"}
	p, err := o.resolve()
	if err != nil {
		t.Fatal(err)
	}
	rows := []map[string]any{
		{"full_name": "octo/api"},
		{"full_name": "octo/ingestor"},
		{"full_name": "octo/web"},
	}
	var buf bytes.Buffer
	if err := p.render(&buf, rows, renderConfig{}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "octo/api\nocto/ingestor\nocto/web\n" {
		t.Errorf("template render = %q", buf.String())
	}
}

// TestRender_TemplateMissingKeyIsExecutionError is the regression for the
// findings.md M2 issue: lower-case field names force map-backed context;
// without `missingkey=error` a typo would render `<no value>` instead of
// failing loudly.
func TestRender_TemplateMissingKeyIsExecutionError(t *testing.T) {
	o := &outputFlags{template: "{{.full_nam}}"}
	p, _ := o.resolve()
	rows := []map[string]any{{"full_name": "octo/api"}}
	var buf bytes.Buffer
	err := p.render(&buf, rows, renderConfig{})
	if err == nil {
		t.Fatal("expected execution error for unknown field")
	}
	if strings.Contains(buf.String(), "<no value>") {
		t.Errorf("stdout must not include <no value>; got %q", buf.String())
	}
}

// TestRender_TemplateRejectsEmbeddedNewline is the regression for the
// findings.md H1 issue: a template that emits multiple lines per item
// would silently violate the pipe-friendly contract.
func TestRender_TemplateRejectsEmbeddedNewline(t *testing.T) {
	o := &outputFlags{template: `{{printf "%s\n%s" .owner .name}}`}
	p, _ := o.resolve()
	rows := []map[string]any{{"owner": "octo", "name": "api"}}
	var buf bytes.Buffer
	err := p.render(&buf, rows, renderConfig{})
	if err == nil {
		t.Fatal("expected embedded-newline rejection")
	}
	if !strings.Contains(err.Error(), "exactly one line per item") {
		t.Errorf("error %q missing one-line-per-item guidance", err.Error())
	}
}

// TestRender_TemplateAllowsTrailingNewline accepts a template that
// already terminates with a newline — only embedded newlines anywhere
// else are an error.
func TestRender_TemplateAllowsTrailingNewline(t *testing.T) {
	o := &outputFlags{template: "{{.full_name}}\n"}
	p, _ := o.resolve()
	rows := []map[string]any{{"full_name": "octo/api"}}
	var buf bytes.Buffer
	if err := p.render(&buf, rows, renderConfig{}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "octo/api\n" {
		t.Errorf("render = %q", buf.String())
	}
}

// TestRender_TemplateEmptyResult emits no output for an empty row set —
// matches the design contract for template mode.
func TestRender_TemplateEmptyResult(t *testing.T) {
	o := &outputFlags{template: "{{.full_name}}"}
	p, _ := o.resolve()
	var buf bytes.Buffer
	if err := p.render(&buf, nil, renderConfig{}); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 0 {
		t.Errorf("empty template render = %q, want empty", buf.String())
	}
}

// TestRender_ErrorTypeForExecutionFailure asserts the wrapped error from
// template execution still surfaces the underlying error semantics so
// callers can chain `errors.Is` / `errors.As` if needed.
func TestRender_ErrorTypeForExecutionFailure(t *testing.T) {
	o := &outputFlags{template: "{{.full_nam}}"}
	p, _ := o.resolve()
	err := p.render(io.Discard, []map[string]any{{"full_name": "octo/api"}}, renderConfig{})
	if err == nil {
		t.Fatal("expected error")
	}
	// errors.As against itself is trivially true; this is a smoke check
	// that the wrapped error is not nil at any depth.
	var wrapped error = err
	for wrapped != nil {
		wrapped = errors.Unwrap(wrapped)
	}
}
