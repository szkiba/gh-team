package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/security"
)

// renderSecurityResult mirrors the tail of runSecuritySummary /
// runSecurityAlerts: render rows via the output plan, then emit
// warnings + exit on hard failures. It lets us assert the partial-
// failure contract without spinning up a fake GitHub client.
func renderSecurityResult(t *testing.T, of *outputFlags, res *security.Result, kind string) (stdout, stderr string, err error) {
	t.Helper()
	plan, perr := of.resolve()
	if perr != nil {
		t.Fatalf("resolve: %v", perr)
	}
	c := &cobra.Command{}
	var outBuf, errBuf bytes.Buffer
	c.SetOut(&outBuf)
	c.SetErr(&errBuf)

	// Mirror the production command order: warnings to stderr first, then
	// render. A render-time failure must not swallow the warnings.
	emitSecurityWarnings(c, res)

	var renderErr error
	switch kind {
	case "summary":
		renderErr = plan.render(c.OutOrStdout(), summaryRows(res.Summary), renderConfig{header: "repo\tfamily\tcount", defFn: renderSummaryDefault})
	case "alerts":
		renderErr = plan.render(c.OutOrStdout(), alertRows(res.Alerts), renderConfig{header: "family\trepo\tkey\tseverity\turl", defFn: renderAlertDefault})
	default:
		t.Fatalf("unknown kind %q", kind)
	}
	if renderErr != nil {
		return outBuf.String(), errBuf.String(), renderErr
	}
	return outBuf.String(), errBuf.String(), securityExitStatus(res)
}

// TestPartialFailure_RenderErrorStillEmitsWarnings is the regression for
// findings.md M1: a template-time failure (here, embedded newline) must
// surface as the returned error, but the collector's per-repo warnings
// must still reach stderr. The pre-fix code rendered first and only
// emitted warnings on the successful-render path, which swallowed
// diagnostics when --template tripped the one-line guard.
func TestPartialFailure_RenderErrorStillEmitsWarnings(t *testing.T) {
	res := &security.Result{
		Alerts: []security.AlertRow{
			{Family: security.FamilyDependabot, Repo: "octo/api",
				Key: "k", Severity: "high", URL: "https://example/u"},
		},
		Warnings:     []string{"warning: cannot read dependabot alerts for octo/web: forbidden"},
		HardFailures: 1,
	}
	_, stderr, err := renderSecurityResult(t,
		&outputFlags{template: `{{printf "%s\n%s" .repo .url}}`}, res, "alerts")
	if err == nil {
		t.Fatal("expected render error from embedded newline")
	}
	if stderr == "" {
		t.Errorf("warnings lost — stderr empty after render failure")
	}
}

// TestPartialFailure_JSONStdoutAndStderrWarnings covers the spec scenario
// "Partial failure with JSON output": stdout still emits valid JSON for
// the rows the collector did get, stderr still includes the per-repo
// warnings, and the command still exits with errSecurityIncomplete so
// automation sees a non-zero status.
func TestPartialFailure_JSONStdoutAndStderrWarnings(t *testing.T) {
	res := &security.Result{
		Alerts: []security.AlertRow{
			{Family: security.FamilyDependabot, Repo: "octo/api",
				Key: "k", Severity: "high", URL: "u"},
		},
		Warnings:     []string{"warning: cannot read dependabot alerts for octo/web: forbidden"},
		HardFailures: 1,
	}
	stdout, stderr, err := renderSecurityResult(t, &outputFlags{json: true}, res, "alerts")
	if err == nil {
		t.Fatal("expected errSecurityIncomplete")
	}
	var ese errSecurityIncomplete
	if !errors.As(err, &ese) {
		t.Errorf("err = %T, want errSecurityIncomplete", err)
	}

	var arr []map[string]any
	if jerr := json.Unmarshal([]byte(stdout), &arr); jerr != nil {
		t.Fatalf("stdout is not valid JSON %q: %v", stdout, jerr)
	}
	if len(arr) != 1 || arr[0]["repo"] != "octo/api" {
		t.Errorf("stdout did not contain successful row: %v", arr)
	}
	if stderr == "" || arr == nil {
		t.Errorf("stderr empty; expected per-repo warning")
	}
}

// TestPartialFailure_TemplateStdoutKeepsWarnings is the template-mode
// twin: the renderer must still write one rendered line per successful
// item, leaving warnings + non-zero exit untouched.
func TestPartialFailure_TemplateStdoutKeepsWarnings(t *testing.T) {
	res := &security.Result{
		Summary: []security.SummaryRow{
			{Repo: "octo/api", Family: security.FamilyDependabot, Count: 2},
		},
		Warnings:     []string{"warning: cannot read code-scanning alerts for octo/web: forbidden"},
		HardFailures: 1,
	}
	stdout, stderr, err := renderSecurityResult(t,
		&outputFlags{template: "{{.repo}} {{.count}}"}, res, "summary")
	if err == nil {
		t.Fatal("expected errSecurityIncomplete")
	}
	if stdout != "octo/api 2\n" {
		t.Errorf("template stdout = %q, want \"octo/api 2\\n\"", stdout)
	}
	if stderr == "" {
		t.Errorf("stderr empty; expected warning to survive into template mode")
	}
}
