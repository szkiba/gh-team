package security

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/szkiba/gh-team/internal/ownership"
)

// prJSON renders a single pull-request JSON object with the fields the
// collector actually decodes. Labels is a `,`-separated string for terse
// test setup; empty means no labels.
func prJSON(repoFull string, number int, title, author, labels, updated string) string {
	labelArr := "[]"
	if labels != "" {
		parts := strings.Split(labels, ",")
		var quoted []string
		for _, p := range parts {
			quoted = append(quoted, fmt.Sprintf(`{"name":%q}`, strings.TrimSpace(p)))
		}
		labelArr = "[" + strings.Join(quoted, ",") + "]"
	}
	return fmt.Sprintf(`{
		"number": %d,
		"state": "open",
		"title": %q,
		"html_url": "https://github.com/%s/pull/%d",
		"updated_at": %q,
		"user": {"login": %q},
		"labels": %s
	}`, number, title, repoFull, number, updated, author, labelArr)
}

func prPage(items ...string) string { return "[" + strings.Join(items, ",") + "]" }

func TestPullMatcher_DefaultsOR(t *testing.T) {
	m := &PullMatcher{Title: DefaultPullTitleRegex(), Labels: []string{DefaultPullLabel}}

	cases := []struct {
		name   string
		title  string
		labels []string
		want   bool
	}{
		{"title prefix", "[security] rotate keys", nil, true},
		{"title prefix case-insensitive", "[Security] rotate keys", nil, true},
		{"title security colon", "security: rotate keys", nil, true},
		{"title suffix", "rotate keys [security]", nil, true},
		{"label only", "routine refactor", []string{"dependencies", "security"}, true},
		{"both", "[security] x", []string{"security"}, true},
		{"neither", "add dark mode", []string{"feature"}, false},
		{"insecurity does not match", "address insecurity", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.Match(tc.title, tc.labels); got != tc.want {
				t.Errorf("Match(%q, %v) = %v, want %v", tc.title, tc.labels, got, tc.want)
			}
		})
	}
}

func TestPullMatcher_OverrideReplacesDefault(t *testing.T) {
	custom := regexp.MustCompile(`^SEC-[0-9]+`)
	m := &PullMatcher{Title: custom, Labels: []string{DefaultPullLabel}}

	if m.Match("[security] x", nil) {
		t.Error("custom title regex must replace the default; expected no match for [security]")
	}
	if !m.Match("SEC-1234 rotate keys", nil) {
		t.Error("custom title regex must match SEC-prefixed titles")
	}
	if !m.Match("anything", []string{"security"}) {
		t.Error("label default must still apply when only --title is overridden")
	}
}

func TestPullMatcher_RepeatedLabelsReplaceDefault(t *testing.T) {
	m := &PullMatcher{Title: DefaultPullTitleRegex(), Labels: []string{"compliance", "audit"}}

	if m.Match("noisy thing", []string{"security"}) {
		t.Error("custom labels must replace the default; expected no match for label=security")
	}
	if !m.Match("noisy thing", []string{"compliance"}) {
		t.Error("expected match against custom label `compliance`")
	}
	if !m.Match("noisy thing", []string{"audit"}) {
		t.Error("expected match against custom label `audit`")
	}
}

func TestPullsCollect_BasicMatchAndSort(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/pulls", staticJSON(prPage(
		prJSON("octo/api", 17, "[security] bump openssl", "alice", "security,dependencies", "2026-05-28T07:30:15Z"),
		prJSON("octo/api", 23, "unrelated refactor", "bob", "", "2026-05-28T08:00:00Z"),
	)))
	c.on("repos/octo/web/pulls", staticJSON(prPage(
		prJSON("octo/web", 4, "routine readme tweak", "carol", "security", "2026-05-27T10:00:00Z"),
	)))

	col := &PullsCollector{
		Client:  c,
		Matcher: &PullMatcher{Title: DefaultPullTitleRegex(), Labels: []string{DefaultPullLabel}},
	}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api"), repo("octo", "web")})
	if err != nil {
		t.Fatal(err)
	}
	if res.HardFailures != 0 {
		t.Errorf("HardFailures = %d, want 0", res.HardFailures)
	}
	want := []PullRequest{
		{Repo: "octo/api", Number: 17, State: "open", Title: "[security] bump openssl",
			Author: "alice", Updated: "2026-05-28T07:30:15Z",
			URL: "https://github.com/octo/api/pull/17"},
		{Repo: "octo/web", Number: 4, State: "open", Title: "routine readme tweak",
			Author: "carol", Updated: "2026-05-27T10:00:00Z",
			URL: "https://github.com/octo/web/pull/4"},
	}
	if !reflect.DeepEqual(res.PullRequests, want) {
		t.Errorf("PullRequests = %+v, want %+v", res.PullRequests, want)
	}
}

func TestPullsCollect_SortNumberDescWithinRepo(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/pulls", staticJSON(prPage(
		prJSON("octo/api", 17, "[security] one", "a", "", "2026-05-28T07:30:15Z"),
		prJSON("octo/api", 23, "[security] two", "b", "", "2026-05-28T08:00:00Z"),
	)))
	c.on("repos/octo/web/pulls", staticJSON(prPage(
		prJSON("octo/web", 4, "[security] three", "c", "", "2026-05-27T10:00:00Z"),
	)))

	col := &PullsCollector{
		Client:  c,
		Matcher: &PullMatcher{Title: DefaultPullTitleRegex(), Labels: []string{DefaultPullLabel}},
	}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api"), repo("octo", "web")})
	if err != nil {
		t.Fatal(err)
	}
	gotOrder := []string{}
	for _, p := range res.PullRequests {
		gotOrder = append(gotOrder, fmt.Sprintf("%s#%d", p.Repo, p.Number))
	}
	want := []string{"octo/api#23", "octo/api#17", "octo/web#4"}
	if !reflect.DeepEqual(gotOrder, want) {
		t.Errorf("order = %v, want %v", gotOrder, want)
	}
}

func TestPullsCollect_Pagination(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/pulls", pagedJSON(
		prPage(prJSON("octo/api", 1, "[security] a", "x", "", "2026-05-28T07:30:15Z")),
		"repos/octo/api/pulls/page2",
	))
	c.on("repos/octo/api/pulls/page2", staticJSON(prPage(
		prJSON("octo/api", 2, "[security] b", "y", "", "2026-05-28T08:30:15Z"))))

	col := &PullsCollector{
		Client:  c,
		Matcher: &PullMatcher{Title: DefaultPullTitleRegex(), Labels: []string{DefaultPullLabel}},
	}
	res, err := col.Collect(context.Background(), []ownership.Repo{repo("octo", "api")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.PullRequests) != 2 {
		t.Fatalf("got %d PRs across pages, want 2: %+v", len(res.PullRequests), res.PullRequests)
	}
}

func TestPullsCollect_AccessDeniedPerRepo(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/pulls", staticJSON(prPage(
		prJSON("octo/api", 17, "[security] ok", "a", "", "2026-05-28T07:30:15Z"))))
	c.on("repos/octo/web/pulls", httpErr(403, "Resource not accessible by integration", nil))

	col := &PullsCollector{
		Client:  c,
		Matcher: &PullMatcher{Title: DefaultPullTitleRegex(), Labels: []string{DefaultPullLabel}},
	}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api"), repo("octo", "web")})
	if err != nil {
		t.Fatal(err)
	}
	if res.HardFailures != 1 {
		t.Errorf("HardFailures = %d, want 1", res.HardFailures)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "octo/web") {
		t.Errorf("expected one warning naming octo/web; got %v", res.Warnings)
	}
	if len(res.PullRequests) != 1 || res.PullRequests[0].Repo != "octo/api" {
		t.Errorf("expected one successful row from octo/api; got %+v", res.PullRequests)
	}
}

// TestPullsCollect_ScopeMissingIsPerRepoNotFatal asserts the H1 fix: a
// 403 with the scope-missing signature on the pulls endpoint is treated
// as a per-repository access failure (warn + exit 1), not a run-wide
// fatal. Cancelling the whole run on the first private-repo scope
// failure would lose public-repo rows the user is entitled to, since
// public repos list without `repo` scope.
func TestPullsCollect_ScopeMissingIsPerRepoNotFatal(t *testing.T) {
	c := newFakeClient()
	headers := http.Header{}
	headers.Set("X-Accepted-Oauth-Scopes", "repo, public_repo")
	headers.Set("X-Oauth-Scopes", "read:org")
	c.on("repos/octo/api/pulls", staticJSON(prPage(
		prJSON("octo/api", 9, "[security] keep", "a", "", "2026-05-28T07:30:15Z"))))
	c.on("repos/octo/private/pulls", httpErr(403, "Resource not accessible by integration", headers))

	col := &PullsCollector{
		Client:  c,
		Matcher: &PullMatcher{Title: DefaultPullTitleRegex(), Labels: []string{DefaultPullLabel}},
	}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api"), repo("octo", "private")})
	if err != nil {
		t.Fatalf("expected non-fatal continuation; got %v", err)
	}
	if res.HardFailures != 1 {
		t.Errorf("HardFailures = %d, want 1", res.HardFailures)
	}
	if len(res.PullRequests) != 1 || res.PullRequests[0].Repo != "octo/api" {
		t.Errorf("expected public-repo row preserved; got %+v", res.PullRequests)
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want 1", res.Warnings)
	}
	for _, want := range []string{"octo/private", "gh auth refresh -s repo"} {
		if !strings.Contains(res.Warnings[0], want) {
			t.Errorf("warning %q missing %q", res.Warnings[0], want)
		}
	}
	if strings.Contains(res.Warnings[0], "security_events") {
		t.Errorf("warning must not recommend security_events: %q", res.Warnings[0])
	}
}

func TestPullsCollect_404IsAccessDenied(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/pulls", httpErr(404, "Not Found", nil))

	col := &PullsCollector{
		Client:  c,
		Matcher: &PullMatcher{Title: DefaultPullTitleRegex(), Labels: []string{DefaultPullLabel}},
	}
	res, err := col.Collect(context.Background(), []ownership.Repo{repo("octo", "api")})
	if err != nil {
		t.Fatal(err)
	}
	if res.HardFailures != 1 {
		t.Errorf("HardFailures = %d, want 1", res.HardFailures)
	}
}

func TestPullsCollect_StateFilterClientSide(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/pulls", staticJSON(`[
		{"number":1,"state":"open","title":"[security] keep","html_url":"u1","updated_at":"2026-05-28T07:30:15Z","user":{"login":"a"},"labels":[]},
		{"number":2,"state":"closed","title":"[security] drop","html_url":"u2","updated_at":"2026-05-28T07:30:15Z","user":{"login":"a"},"labels":[]}
	]`))

	col := &PullsCollector{
		Client:  c,
		Matcher: &PullMatcher{Title: DefaultPullTitleRegex(), Labels: []string{DefaultPullLabel}},
	}
	res, err := col.Collect(context.Background(), []ownership.Repo{repo("octo", "api")})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.PullRequests) != 1 || res.PullRequests[0].Number != 1 {
		t.Errorf("expected only the open PR; got %+v", res.PullRequests)
	}
}

func TestNormalizeUpdated(t *testing.T) {
	cases := map[string]string{
		"2026-05-28T07:30:15Z":      "2026-05-28T07:30:15Z",
		"2026-05-28T07:30:15+02:00": "2026-05-28T05:30:15Z",
		"":                          "",
		"not-a-date":                "not-a-date",
	}
	for in, want := range cases {
		if got := normalizeUpdated(in); got != want {
			t.Errorf("normalizeUpdated(%q) = %q, want %q", in, got, want)
		}
	}
}
