package cmd

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/szkiba/gh-team/internal/security"
)

// renderSummaryWithPlan is a thin test harness around the shared output
// helpers. It builds a Result-like slice and routes it through the same
// (plan, rows, default-renderer) tuple the command uses, so we exercise
// the public contract end-to-end without touching the GitHub REST layer.
func renderSummaryWithPlan(t *testing.T, of *outputFlags, sum []security.SummaryRow) string {
	t.Helper()
	plan, err := of.resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var buf bytes.Buffer
	if err := plan.render(&buf, summaryRows(sum), renderConfig{header: "repo\tfamily\tcount", defFn: renderSummaryDefault}); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

// TestSecuritySummary_DefaultByteCompat verifies the default-mode output
// is unchanged from the pre-flag implementation (tab-separated, sorted
// by repo then family, no headers).
func TestSecuritySummary_DefaultByteCompat(t *testing.T) {
	rows := []security.SummaryRow{
		{Repo: "octo/api", Family: security.FamilyCodeScanning, Count: 1},
		{Repo: "octo/api", Family: security.FamilyDependabot, Count: 2},
		{Repo: "octo/web", Family: security.FamilyCodeScanning, Count: 3},
	}
	got := renderSummaryWithPlan(t, &outputFlags{}, rows)
	want := "octo/api\tcode-scanning\t1\nocto/api\tdependabot\t2\nocto/web\tcode-scanning\t3\n"
	if got != want {
		t.Errorf("default summary output drifted:\n got %q\nwant %q", got, want)
	}
}

// TestSecuritySummary_JSONMatchesSpec covers field names and ordering for
// `--json`: each item must expose .repo, .family, .count, and the array
// must follow the same repo-then-family order as default mode.
func TestSecuritySummary_JSONMatchesSpec(t *testing.T) {
	rows := []security.SummaryRow{
		{Repo: "octo/api", Family: security.FamilyDependabot, Count: 2},
		{Repo: "octo/web", Family: security.FamilyCodeScanning, Count: 3},
	}
	got := renderSummaryWithPlan(t, &outputFlags{json: true}, rows)
	var arr []map[string]any
	if err := json.Unmarshal([]byte(got), &arr); err != nil {
		t.Fatalf("invalid JSON %q: %v", got, err)
	}
	if len(arr) != 2 {
		t.Fatalf("got %d items, want 2", len(arr))
	}
	if arr[0]["repo"] != "octo/api" || arr[1]["repo"] != "octo/web" {
		t.Errorf("ordering not preserved: %v", arr)
	}
	for i, item := range arr {
		for _, k := range []string{"repo", "family", "count"} {
			if _, ok := item[k]; !ok {
				t.Errorf("item %d missing field %q: %v", i, k, item)
			}
		}
	}
}

// TestSecuritySummary_TemplatePreservesDefaultOrdering is the explicit
// regression for the spec scenario `Template summary preserves default
// ordering`: template-mode rendering must follow the same sort as
// default mode.
func TestSecuritySummary_TemplatePreservesDefaultOrdering(t *testing.T) {
	rows := []security.SummaryRow{
		{Repo: "octo/api", Family: security.FamilyCodeScanning, Count: 1},
		{Repo: "octo/api", Family: security.FamilyDependabot, Count: 2},
		{Repo: "octo/web", Family: security.FamilyCodeScanning, Count: 3},
	}
	got := renderSummaryWithPlan(t, &outputFlags{template: "{{.repo}}/{{.family}}"}, rows)
	want := "octo/api/code-scanning\nocto/api/dependabot\nocto/web/code-scanning\n"
	if got != want {
		t.Errorf("template order mismatch:\n got %q\nwant %q", got, want)
	}
}

// TestSummaryRows_FieldShape locks the projection from the collector's
// internal SummaryRow into the public template/JSON map. Catching drift
// here protects automation that pinned to these field names.
func TestSummaryRows_FieldShape(t *testing.T) {
	rows := summaryRows([]security.SummaryRow{
		{Repo: "octo/api", Family: security.FamilyDependabot, Count: 5},
	})
	want := []map[string]any{
		{"repo": "octo/api", "family": "dependabot", "count": 5},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
}

// TestSecuritySummary_TemplateMissingKey is wired through summaryRows so
// the missingkey=error behavior survives the row projection — a user
// typo against the lower-case JSON contract must fail loudly.
func TestSecuritySummary_TemplateMissingKey(t *testing.T) {
	rows := []security.SummaryRow{
		{Repo: "octo/api", Family: security.FamilyDependabot, Count: 2},
	}
	plan, err := (&outputFlags{template: "{{.cnt}}"}).resolve()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := plan.render(&buf, summaryRows(rows), renderConfig{header: "repo\tfamily\tcount", defFn: renderSummaryDefault}); err == nil {
		t.Fatal("expected execution error for unknown field .cnt")
	}
	if strings.Contains(buf.String(), "<no value>") {
		t.Errorf("stdout leaked <no value>: %q", buf.String())
	}
}
