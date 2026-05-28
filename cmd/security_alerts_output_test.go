package cmd

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/szkiba/gh-team/internal/security"
)

func renderAlertsWithPlan(t *testing.T, of *outputFlags, a []security.AlertRow) string {
	t.Helper()
	plan, err := of.resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var buf bytes.Buffer
	if err := plan.render(&buf, alertRows(a), renderAlertDefault); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestSecurityAlerts_DefaultByteCompat(t *testing.T) {
	rows := []security.AlertRow{
		{Family: security.FamilyCodeScanning, Repo: "octo/api", Key: "go/sql-injection",
			Severity: "high", URL: "https://example/cs/4"},
		{Family: security.FamilyDependabot, Repo: "octo/api",
			Key: "npm:lodash@/web/package-lock.json", Severity: "high",
			URL: "https://example/dep/7"},
	}
	got := renderAlertsWithPlan(t, &outputFlags{}, rows)
	want := "code-scanning\tocto/api\tgo/sql-injection\thigh\thttps://example/cs/4\n" +
		"dependabot\tocto/api\tnpm:lodash@/web/package-lock.json\thigh\thttps://example/dep/7\n"
	if got != want {
		t.Errorf("default alerts output drifted:\n got %q\nwant %q", got, want)
	}
}

func TestSecurityAlerts_JSONShape(t *testing.T) {
	rows := []security.AlertRow{
		{Family: security.FamilyDependabot, Repo: "octo/api", Key: "k", Severity: "high", URL: "u"},
	}
	got := renderAlertsWithPlan(t, &outputFlags{json: true}, rows)
	var arr []map[string]any
	if err := json.Unmarshal([]byte(got), &arr); err != nil {
		t.Fatalf("invalid JSON %q: %v", got, err)
	}
	if len(arr) != 1 {
		t.Fatalf("got %d items, want 1", len(arr))
	}
	for _, k := range []string{"family", "repo", "key", "severity", "url"} {
		if _, ok := arr[0][k]; !ok {
			t.Errorf("item missing field %q: %v", k, arr[0])
		}
	}
}

// TestSecurityAlerts_TemplatePreservesDefaultOrdering covers the spec
// scenario `Template alerts preserve default ordering`.
func TestSecurityAlerts_TemplatePreservesDefaultOrdering(t *testing.T) {
	rows := []security.AlertRow{
		{Family: security.FamilyCodeScanning, Repo: "octo/api", Key: "a", Severity: "high", URL: "u1"},
		{Family: security.FamilyDependabot, Repo: "octo/api", Key: "b", Severity: "low", URL: "u2"},
		{Family: security.FamilyCodeScanning, Repo: "octo/web", Key: "c", Severity: "low", URL: "u3"},
	}
	got := renderAlertsWithPlan(t, &outputFlags{template: "{{.repo}} {{.family}} {{.key}}"}, rows)
	want := "octo/api code-scanning a\nocto/api dependabot b\nocto/web code-scanning c\n"
	if got != want {
		t.Errorf("template order mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestAlertRows_FieldShape(t *testing.T) {
	rows := alertRows([]security.AlertRow{
		{Family: security.FamilyDependabot, Repo: "octo/api",
			Key: "npm:lodash@x", Severity: "high", URL: "https://x"},
	})
	want := []map[string]any{
		{
			"family":   "dependabot",
			"repo":     "octo/api",
			"key":      "npm:lodash@x",
			"severity": "high",
			"url":      "https://x",
		},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
}
