package cmd

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/ownership"
	"github.com/szkiba/gh-team/internal/security"
)

// TestResolve_HeaderRejectedWithJSON enforces the team-cli spec scenario
// `--header conflicts with --json`: --header is a default-mode modifier
// and combining it with --json would break the single-JSON-array contract.
func TestResolve_HeaderRejectedWithJSON(t *testing.T) {
	o := &outputFlags{header: true, json: true}
	_, err := o.resolve()
	if err == nil {
		t.Fatal("expected rejection")
	}
	for _, want := range []string{"--header", "--json"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

// TestResolve_HeaderRejectedWithTemplate covers the matching `--header
// conflicts with --template` scenario.
func TestResolve_HeaderRejectedWithTemplate(t *testing.T) {
	o := &outputFlags{header: true, template: "{{.full_name}}"}
	_, err := o.resolve()
	if err == nil {
		t.Fatal("expected rejection")
	}
	for _, want := range []string{"--header", "--template"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err.Error(), want)
		}
	}
}

// TestResolve_HeaderDefaultModePlan confirms the resolved plan carries the
// header flag through to render time when neither output mode is set.
func TestResolve_HeaderDefaultModePlan(t *testing.T) {
	o := &outputFlags{header: true}
	p, err := o.resolve()
	if err != nil {
		t.Fatal(err)
	}
	if p.mode != outputDefault {
		t.Errorf("mode = %v, want outputDefault", p.mode)
	}
	if !p.header {
		t.Error("plan.header = false, want true")
	}
}

// TestRepoList_HeaderWidensRowsToFourColumns is the core spec scenario for
// `repo list` from team-repo/spec.md — when --header is set, the rows
// also switch to the four-column TSV shape so the header actually
// describes the data below it.
func TestRepoList_HeaderWidensRowsToFourColumns(t *testing.T) {
	repos := []ownership.Repo{
		{Owner: "octo", Name: "api", Archived: false},
		{Owner: "octo", Name: "legacy", Archived: true},
	}
	o := &outputFlags{header: true}
	plan, err := o.resolve()
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := plan.render(&buf, repoRows(repos), renderConfig{
		header:      "owner\tname\tfull_name\tarchived",
		defFn:       renderRepoDefault,
		defHeaderFn: renderRepoWithHeaderColumns,
	}); err != nil {
		t.Fatal(err)
	}
	want := "owner\tname\tfull_name\tarchived\n" +
		"octo\tapi\tocto/api\tfalse\n" +
		"octo\tlegacy\tocto/legacy\ttrue\n"
	if buf.String() != want {
		t.Errorf("render = %q, want %q", buf.String(), want)
	}
}

// TestRepoList_NoHeaderKeepsSingleColumn confirms the v0.3.0 byte
// contract still holds for the no-flag path even though the helper now
// knows how to widen rows. Default mode without --header must emit one
// `<org>/<repo>` per line.
func TestRepoList_NoHeaderKeepsSingleColumn(t *testing.T) {
	repos := []ownership.Repo{
		{Owner: "octo", Name: "api"},
		{Owner: "octo", Name: "web"},
	}
	o := &outputFlags{}
	plan, _ := o.resolve()
	var buf bytes.Buffer
	if err := plan.render(&buf, repoRows(repos), renderConfig{
		header:      "owner\tname\tfull_name\tarchived",
		defFn:       renderRepoDefault,
		defHeaderFn: renderRepoWithHeaderColumns,
	}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "octo/api\nocto/web\n" {
		t.Errorf("no-header render = %q", buf.String())
	}
}

// TestRepoList_HeaderEmitsWithoutDataRows covers the empty-result spec
// scenario: --header still emits the column-name line so spreadsheet
// importers see the header.
func TestRepoList_HeaderEmitsWithoutDataRows(t *testing.T) {
	o := &outputFlags{header: true}
	plan, _ := o.resolve()
	var buf bytes.Buffer
	if err := plan.render(&buf, repoRows(nil), renderConfig{
		header:      "owner\tname\tfull_name\tarchived",
		defFn:       renderRepoDefault,
		defHeaderFn: renderRepoWithHeaderColumns,
	}); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "owner\tname\tfull_name\tarchived\n" {
		t.Errorf("empty-result header = %q", buf.String())
	}
}

// TestSecuritySummary_HeaderContent verifies the summary header is exactly
// `repo\tfamily\tcount\n` and the rows below it remain the existing
// 3-column default-mode TSV.
func TestSecuritySummary_HeaderContent(t *testing.T) {
	rows := []security.SummaryRow{
		{Repo: "octo/api", Family: security.FamilyCodeScanning, Count: 1},
		{Repo: "octo/api", Family: security.FamilyDependabot, Count: 2},
	}
	o := &outputFlags{header: true}
	plan, _ := o.resolve()
	var buf bytes.Buffer
	if err := plan.render(&buf, summaryRows(rows), renderConfig{
		header: "repo\tfamily\tcount",
		defFn:  renderSummaryDefault,
	}); err != nil {
		t.Fatal(err)
	}
	want := "repo\tfamily\tcount\n" +
		"octo/api\tcode-scanning\t1\n" +
		"octo/api\tdependabot\t2\n"
	if buf.String() != want {
		t.Errorf("summary render = %q, want %q", buf.String(), want)
	}
}

// TestSecurityAlerts_HeaderContent verifies the alerts header is exactly
// `family\trepo\tkey\tseverity\turl\n`.
func TestSecurityAlerts_HeaderContent(t *testing.T) {
	rows := []security.AlertRow{
		{Family: security.FamilyCodeScanning, Repo: "octo/api", Key: "go/sql-injection",
			Severity: "high", URL: "https://example/4"},
	}
	o := &outputFlags{header: true}
	plan, _ := o.resolve()
	var buf bytes.Buffer
	if err := plan.render(&buf, alertRows(rows), renderConfig{
		header: "family\trepo\tkey\tseverity\turl",
		defFn:  renderAlertDefault,
	}); err != nil {
		t.Fatal(err)
	}
	want := "family\trepo\tkey\tseverity\turl\n" +
		"code-scanning\tocto/api\tgo/sql-injection\thigh\thttps://example/4\n"
	if buf.String() != want {
		t.Errorf("alerts render = %q, want %q", buf.String(), want)
	}
}

// TestSecurityAlerts_HeaderSurvivesPartialFailure covers the team-security
// spec scenario: with --header and an access failure on one repo, the
// header line still leads stdout, the warning still reaches stderr, and
// the run exits non-zero.
func TestSecurityAlerts_HeaderSurvivesPartialFailure(t *testing.T) {
	res := &security.Result{
		Alerts: []security.AlertRow{
			{Family: security.FamilyDependabot, Repo: "octo/api",
				Key: "k", Severity: "high", URL: "https://x"},
		},
		Warnings:     []string{"warning: cannot read dependabot alerts for octo/web: forbidden"},
		HardFailures: 1,
	}
	o := &outputFlags{header: true}
	plan, _ := o.resolve()
	c := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	c.SetOut(&stdout)
	c.SetErr(&stderr)

	emitSecurityWarnings(c, res)
	if err := plan.render(c.OutOrStdout(), alertRows(res.Alerts), renderConfig{
		header: "family\trepo\tkey\tseverity\turl",
		defFn:  renderAlertDefault,
	}); err != nil {
		t.Fatalf("render: %v", err)
	}
	exitErr := securityExitStatus(res)

	if !strings.HasPrefix(stdout.String(), "family\trepo\tkey\tseverity\turl\n") {
		t.Errorf("stdout missing header prefix: %q", stdout.String())
	}
	if !strings.Contains(stderr.String(), "octo/web") {
		t.Errorf("stderr missing partial-failure warning: %q", stderr.String())
	}
	var ese errSecurityIncomplete
	if !errors.As(exitErr, &ese) {
		t.Errorf("expected errSecurityIncomplete, got %v", exitErr)
	}
}
