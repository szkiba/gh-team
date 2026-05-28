package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/security"
)

func renderPullsWithPlan(t *testing.T, of *outputFlags, rows []security.PullRequest) string {
	t.Helper()
	plan, err := of.resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var buf bytes.Buffer
	if err := plan.render(&buf, pullRows(rows), renderConfig{
		header: "repo\tnumber\tstate\ttitle\tauthor\tupdated\turl",
		defFn:  renderPullDefault,
	}); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func samplePulls() []security.PullRequest {
	return []security.PullRequest{
		{Repo: "octo/api", Number: 17, State: "open",
			Title: "[security] bump openssl", Author: "alice",
			Updated: "2026-05-28T07:30:15Z",
			URL:     "https://github.com/octo/api/pull/17"},
		{Repo: "octo/web", Number: 4, State: "open",
			Title: "routine readme tweak", Author: "carol",
			Updated: "2026-05-27T10:00:00Z",
			URL:     "https://github.com/octo/web/pull/4"},
	}
}

func TestSecurityPrs_DefaultByteCompat(t *testing.T) {
	got := renderPullsWithPlan(t, &outputFlags{}, samplePulls())
	want := "octo/api\t17\topen\t[security] bump openssl\talice\t2026-05-28T07:30:15Z\thttps://github.com/octo/api/pull/17\n" +
		"octo/web\t4\topen\troutine readme tweak\tcarol\t2026-05-27T10:00:00Z\thttps://github.com/octo/web/pull/4\n"
	if got != want {
		t.Errorf("default prs output drifted:\n got %q\nwant %q", got, want)
	}
}

func TestSecurityPrs_HeaderLine(t *testing.T) {
	got := renderPullsWithPlan(t, &outputFlags{header: true}, samplePulls())
	if !strings.HasPrefix(got, "repo\tnumber\tstate\ttitle\tauthor\tupdated\turl\n") {
		t.Errorf("expected header line first; got %q", got)
	}
}

func TestSecurityPrs_HeaderEmitsOnEmpty(t *testing.T) {
	got := renderPullsWithPlan(t, &outputFlags{header: true}, nil)
	want := "repo\tnumber\tstate\ttitle\tauthor\tupdated\turl\n"
	if got != want {
		t.Errorf("empty header output = %q, want %q", got, want)
	}
}

func TestSecurityPrs_TabInTitleSanitizedDefault(t *testing.T) {
	rows := []security.PullRequest{
		{Repo: "octo/api", Number: 1, State: "open",
			Title: "[security]\tweird\ttitle", Author: "a",
			Updated: "2026-05-28T07:30:15Z", URL: "https://x"},
	}
	got := renderPullsWithPlan(t, &outputFlags{}, rows)
	if strings.Count(got, "\t") != 6 {
		t.Errorf("expected exactly 6 tabs in default row, got %d in %q", strings.Count(got, "\t"), got)
	}
	if !strings.Contains(got, "[security] weird title") {
		t.Errorf("expected sanitized title %q in %q", "[security] weird title", got)
	}
}

func TestSecurityPrs_NewlineInTitleSanitizedDefault(t *testing.T) {
	rows := []security.PullRequest{
		{Repo: "octo/api", Number: 1, State: "open",
			Title: "[security]\nweird\ntitle", Author: "a",
			Updated: "2026-05-28T07:30:15Z", URL: "https://x"},
	}
	got := renderPullsWithPlan(t, &outputFlags{}, rows)
	if strings.Count(got, "\n") != 1 {
		t.Errorf("expected exactly one trailing newline in default row, got %d in %q",
			strings.Count(got, "\n"), got)
	}
}

func TestSecurityPrs_JSONPreservesTitleVerbatim(t *testing.T) {
	rows := []security.PullRequest{
		{Repo: "octo/api", Number: 1, State: "open",
			Title: "[security]\tweird", Author: "a",
			Updated: "2026-05-28T07:30:15Z", URL: "https://x"},
	}
	got := renderPullsWithPlan(t, &outputFlags{json: true}, rows)
	var arr []map[string]any
	if err := json.Unmarshal([]byte(got), &arr); err != nil {
		t.Fatalf("invalid JSON %q: %v", got, err)
	}
	if len(arr) != 1 {
		t.Fatalf("got %d items, want 1", len(arr))
	}
	if arr[0]["title"] != "[security]\tweird" {
		t.Errorf("JSON title sanitized; expected verbatim, got %q", arr[0]["title"])
	}
}

func TestSecurityPrs_JSONNumberIsInt(t *testing.T) {
	got := renderPullsWithPlan(t, &outputFlags{json: true}, samplePulls())
	if !strings.Contains(got, `"number":17`) {
		t.Errorf("expected `\"number\":17` (integer, not stringified) in %q", got)
	}
}

func TestSecurityPrs_TemplatePreservesDefaultOrdering(t *testing.T) {
	rows := []security.PullRequest{
		{Repo: "octo/api", Number: 23, Title: "two"},
		{Repo: "octo/api", Number: 17, Title: "one"},
		{Repo: "octo/web", Number: 4, Title: "three"},
	}
	got := renderPullsWithPlan(t,
		&outputFlags{template: "{{.repo}}#{{.number}} {{.title}}"}, rows)
	want := "octo/api#23 two\nocto/api#17 one\nocto/web#4 three\n"
	if got != want {
		t.Errorf("template order mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestPullRows_FieldShape(t *testing.T) {
	rows := pullRows([]security.PullRequest{
		{Repo: "octo/api", Number: 42, State: "open", Title: "t",
			Author: "alice", Updated: "2026-05-28T07:30:15Z", URL: "u"},
	})
	want := []map[string]any{
		{
			"repo":    "octo/api",
			"number":  42,
			"state":   "open",
			"title":   "t",
			"author":  "alice",
			"updated": "2026-05-28T07:30:15Z",
			"url":     "u",
		},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
}

func TestBuildPullMatcher_DefaultsWhenNoFlags(t *testing.T) {
	m, err := buildPullMatcher("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if !m.Match("[security] x", nil) {
		t.Error("default title regex should match `[security] x`")
	}
	if !m.Match("noise", []string{"security"}) {
		t.Error("default label should match `security`")
	}
	if m.Match("noise", []string{"feature"}) {
		t.Error("default matcher should not match unrelated PR")
	}
}

func TestBuildPullMatcher_TitleOverrideReplacesDefault(t *testing.T) {
	m, err := buildPullMatcher("^SEC-[0-9]+", nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Match("[security] x", nil) {
		t.Error("default title regex should be replaced by --title override")
	}
	if !m.Match("SEC-1 fix", nil) {
		t.Error("custom title regex should match")
	}
	if !m.Match("anything", []string{"security"}) {
		t.Error("default label still active when only --title is set")
	}
}

func TestBuildPullMatcher_LabelOverrideReplacesDefault(t *testing.T) {
	m, err := buildPullMatcher("", []string{"compliance", "audit"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Match("noise", []string{"security"}) {
		t.Error("default label should be replaced by --label overrides")
	}
	if !m.Match("noise", []string{"compliance"}) {
		t.Error("custom label `compliance` should match")
	}
}

func TestBuildPullMatcher_InvalidTitleRegex(t *testing.T) {
	_, err := buildPullMatcher("[", nil)
	if err == nil {
		t.Fatal("expected compile error from invalid regex")
	}
	if !strings.Contains(err.Error(), `"["`) {
		t.Errorf("err = %q, expected to name the offending pattern", err)
	}
}

func TestBuildPullMatcher_TitleAndLabelOverridesBoth(t *testing.T) {
	m, err := buildPullMatcher("^x", []string{"y"})
	if err != nil {
		t.Fatal(err)
	}
	if m.Match("[security] x", nil) {
		t.Error("both defaults should be replaced when both flags set")
	}
	if m.Match("noise", []string{"security"}) {
		t.Error("default label should be replaced")
	}
	if !m.Match("x stuff", nil) || !m.Match("noise", []string{"y"}) {
		t.Error("custom title or label should match")
	}
}

func TestSecurityPrs_OutputFlagConflicts(t *testing.T) {
	cases := []struct {
		name string
		of   outputFlags
	}{
		{"json+template", outputFlags{json: true, template: "{{.title}}"}},
		{"header+json", outputFlags{header: true, json: true}},
		{"header+template", outputFlags{header: true, template: "{{.title}}"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := tc.of.resolve()
			if err == nil {
				t.Errorf("expected conflict error for %s", tc.name)
			}
		})
	}
}

// TestSecurityPrs_PartialFailureRendersAndExits mirrors the partial-failure
// pattern test for alerts: warnings reach stderr before render, render
// succeeds for the row we have, and the run still exits non-zero.
func TestSecurityPrs_PartialFailureRendersAndExits(t *testing.T) {
	res := &security.PullsResult{
		PullRequests: []security.PullRequest{
			{Repo: "octo/api", Number: 17, State: "open",
				Title: "[security] ok", Author: "a",
				Updated: "2026-05-28T07:30:15Z",
				URL:     "https://github.com/octo/api/pull/17"},
		},
		Warnings:     []string{"warning: cannot read pull requests for octo/web: forbidden"},
		HardFailures: 1,
	}
	plan, err := (&outputFlags{header: true}).resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	c := &cobra.Command{}
	var outBuf, errBuf bytes.Buffer
	c.SetOut(&outBuf)
	c.SetErr(&errBuf)

	emitPullsWarnings(c, res)
	if rerr := plan.render(c.OutOrStdout(), pullRows(res.PullRequests),
		renderConfig{header: "repo\tnumber\tstate\ttitle\tauthor\tupdated\turl", defFn: renderPullDefault}); rerr != nil {
		t.Fatalf("render: %v", rerr)
	}
	if !strings.Contains(outBuf.String(), "octo/api\t17") {
		t.Errorf("stdout missing successful row: %q", outBuf.String())
	}
	if !strings.Contains(errBuf.String(), "octo/web") {
		t.Errorf("stderr missing per-repo warning: %q", errBuf.String())
	}
	if perr := pullsExitStatus(res); perr == nil {
		t.Error("expected non-zero exit status carrier")
	} else {
		var ese errSecurityIncomplete
		if !errors.As(perr, &ese) {
			t.Errorf("exit error = %T, want errSecurityIncomplete", perr)
		}
	}
}

// TestSecurityPrs_DefaultRegexIsCompileable guards the spec'd default
// regex stays valid Go syntax — a regression here would prevent the
// command from running at all.
func TestSecurityPrs_DefaultRegexIsCompileable(t *testing.T) {
	re := security.DefaultPullTitleRegex()
	if re == nil {
		t.Fatal("default regex is nil")
	}
	// Confirm one of the documented sample inputs matches so a future
	// silent rewrite of the regex string is caught.
	if !re.MatchString("[security] rotate keys") {
		t.Error("default regex should match `[security] rotate keys`")
	}
}

// TestSecurityPrs_OverrideEmptyTitleFlagUsesDefault — empty string from
// cobra StringVar default means "no override". Confirm the regex is
// usable rather than stuck at a no-op.
func TestSecurityPrs_OverrideEmptyTitleFlagUsesDefault(t *testing.T) {
	m, err := buildPullMatcher("", nil)
	if err != nil {
		t.Fatal(err)
	}
	if m.Title == nil || m.Title.String() != security.DefaultPullTitleRegex().String() {
		t.Errorf("expected default regex when --title is empty; got %v", m.Title)
	}
}

// TestSecurityPrs_OverrideExplicitRegexCompiles is a small sanity check
// that the parser surfaces the same regexp object the matcher uses.
func TestSecurityPrs_OverrideExplicitRegexCompiles(t *testing.T) {
	m, err := buildPullMatcher(`^SEC-\d+`, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := regexp.MustCompile(`^SEC-\d+`)
	if m.Title.String() != want.String() {
		t.Errorf("override regex = %q, want %q", m.Title.String(), want.String())
	}
}
