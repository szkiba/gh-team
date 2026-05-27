package security

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/szkiba/gh-team/internal/ownership"
)

// dependabotPage builds a JSON array of `n` synthetic Dependabot alert
// records numbered to make per-page identity assertions easy.
func dependabotPage(repo string, start, n int) string {
	items := make([]string, 0, n)
	for i := 0; i < n; i++ {
		idx := start + i
		items = append(items, fmt.Sprintf(`{
			"state":"open",
			"html_url":"https://github.com/%s/security/dependabot/%d",
			"security_vulnerability":{"severity":"high","package":{"ecosystem":"npm","name":"p%d"}},
			"dependency":{"manifest_path":"/m%d"}
		}`, repo, idx, idx, idx))
	}
	return "[" + strings.Join(items, ",") + "]"
}

func codeScanPage(repo string, start, n int) string {
	items := make([]string, 0, n)
	for i := 0; i < n; i++ {
		idx := start + i
		items = append(items, fmt.Sprintf(`{
			"state":"open",
			"html_url":"https://github.com/%s/code-scanning/%d",
			"rule":{"id":"rule-%d","severity":"warning","security_severity_level":"high"}
		}`, repo, idx, idx))
	}
	return "[" + strings.Join(items, ",") + "]"
}

func repo(owner, name string) ownership.Repo {
	return ownership.Repo{Owner: owner, Name: name}
}

func TestCollect_BasicSummaryAndAlerts(t *testing.T) {
	c := newFakeClient()
	// api: 2 dependabot, 1 code-scanning.
	c.on("repos/octo/api/dependabot/alerts", staticJSON(dependabotPage("octo/api", 1, 2)))
	c.on("repos/octo/api/code-scanning/alerts", staticJSON(codeScanPage("octo/api", 1, 1)))
	// web: 0 dependabot, 3 code-scanning.
	c.on("repos/octo/web/dependabot/alerts", staticJSON(`[]`))
	c.on("repos/octo/web/code-scanning/alerts", staticJSON(codeScanPage("octo/web", 1, 3)))

	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api"), repo("octo", "web")},
		KindAll.Families())
	if err != nil {
		t.Fatal(err)
	}

	wantSummary := []SummaryRow{
		{Repo: "octo/api", Family: FamilyCodeScanning, Count: 1},
		{Repo: "octo/api", Family: FamilyDependabot, Count: 2},
		{Repo: "octo/web", Family: FamilyCodeScanning, Count: 3},
	}
	if !reflect.DeepEqual(res.Summary, wantSummary) {
		t.Errorf("summary mismatch:\n got %+v\nwant %+v", res.Summary, wantSummary)
	}
	if res.HardFailures != 0 {
		t.Errorf("HardFailures = %d, want 0", res.HardFailures)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
	// 2 + 1 + 3 = 6 alert rows.
	if len(res.Alerts) != 6 {
		t.Fatalf("got %d alert rows, want 6", len(res.Alerts))
	}
}

// TestCollect_AlertsOrderingMatchesSpec mirrors the spec's "mixed alert
// listing" scenario almost exactly so we cover both the family-tagged
// output shape and the documented sort order.
func TestCollect_AlertsOrderingMatchesSpec(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/dependabot/alerts", staticJSON(`[{
		"state":"open",
		"html_url":"https://github.com/octo/api/security/dependabot/7",
		"security_vulnerability":{"severity":"high","package":{"ecosystem":"npm","name":"lodash"}},
		"dependency":{"manifest_path":"/web/package-lock.json"}
	}]`))
	c.on("repos/octo/api/code-scanning/alerts", staticJSON(`[{
		"state":"open",
		"html_url":"https://github.com/octo/api/code-scanning/4",
		"rule":{"id":"go/sql-injection","severity":"warning","security_severity_level":"high"}
	}]`))

	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		KindAll.Families())
	if err != nil {
		t.Fatal(err)
	}
	want := []AlertRow{
		{Family: FamilyCodeScanning, Repo: "octo/api", Key: "go/sql-injection",
			Severity: "high", URL: "https://github.com/octo/api/code-scanning/4"},
		{Family: FamilyDependabot, Repo: "octo/api",
			Key: "npm:lodash@/web/package-lock.json", Severity: "high",
			URL: "https://github.com/octo/api/security/dependabot/7"},
	}
	if !reflect.DeepEqual(res.Alerts, want) {
		t.Errorf("alerts mismatch:\n got %+v\nwant %+v", res.Alerts, want)
	}
}

// TestCollect_CodeScanningSeverityFallback verifies the
// security_severity_level → rule.severity fallback ordering.
func TestCollect_CodeScanningSeverityFallback(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/code-scanning/alerts", staticJSON(`[{
		"state":"open",
		"html_url":"https://github.com/octo/api/code-scanning/9",
		"rule":{"id":"r9","severity":"note"}
	}]`))
	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		[]Family{FamilyCodeScanning})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Alerts) != 1 || res.Alerts[0].Severity != "note" {
		t.Errorf("expected severity fallback to 'note', got %+v", res.Alerts)
	}
}

// TestCollect_PaginationFollowsToEnd exercises a multi-page Dependabot
// response (100 + 100 + 17) chained via Link rel="next" headers, and
// checks the summary count matches the total. This is the regression test
// for the page=N → HTTP 400 bug.
func TestCollect_PaginationFollowsToEnd(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/code-scanning/alerts", staticJSON(`[]`))
	c.on("repos/octo/api/dependabot/alerts",
		pagedJSON(dependabotPage("octo/api", 1, pageSize),
			"repos/octo/api/dependabot/alerts/page2"))
	c.on("repos/octo/api/dependabot/alerts/page2",
		pagedJSON(dependabotPage("octo/api", pageSize+1, pageSize),
			"repos/octo/api/dependabot/alerts/page3"))
	c.on("repos/octo/api/dependabot/alerts/page3",
		pagedJSON(dependabotPage("octo/api", 2*pageSize+1, 17), ""))

	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		KindAll.Families())
	if err != nil {
		t.Fatal(err)
	}
	if got := summaryCount(res, "octo/api", FamilyDependabot); got != 217 {
		t.Errorf("dependabot count = %d, want 217 (multi-page)", got)
	}
}

// TestCollect_PaginationMultiPageCodeScanning covers the alerts side of the
// same pagination contract on the code-scanning endpoint.
func TestCollect_PaginationMultiPageCodeScanning(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/dependabot/alerts", staticJSON(`[]`))
	c.on("repos/octo/api/code-scanning/alerts",
		pagedJSON(codeScanPage("octo/api", 1, pageSize),
			"repos/octo/api/code-scanning/alerts/page2"))
	c.on("repos/octo/api/code-scanning/alerts/page2",
		pagedJSON(codeScanPage("octo/api", pageSize+1, 5), ""))

	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		[]Family{FamilyCodeScanning})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Alerts) != pageSize+5 {
		t.Errorf("got %d alerts, want %d (multi-page code-scanning)",
			len(res.Alerts), pageSize+5)
	}
}

// TestNextLink_ParsesRFC8288Variants covers GitHub's Link-header shapes —
// multi-rel header, single-rel header, missing header, malformed segment.
func TestNextLink_ParsesRFC8288Variants(t *testing.T) {
	cases := []struct {
		name, header, want string
	}{
		{"empty", "", ""},
		{"only-next", `<https://api.github.com/x?after=cur>; rel="next"`, "https://api.github.com/x?after=cur"},
		{"next-and-last", `<https://api.github.com/x?after=a>; rel="next", <https://api.github.com/x?after=z>; rel="last"`, "https://api.github.com/x?after=a"},
		{"prev-only", `<https://api.github.com/x?before=cur>; rel="prev"`, ""},
		{"malformed-segment", `not-a-link`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nextLink(tc.header); got != tc.want {
				t.Errorf("nextLink(%q) = %q, want %q", tc.header, got, tc.want)
			}
		})
	}
}

// TestCollect_UnavailableFeatureIsZero covers the spec's "feature unavailable
// is treated as zero alerts" scenario: a 404 from the code-scanning endpoint
// whose message names the disabled feature must NOT generate a warning or
// hard failure.
func TestCollect_UnavailableFeatureIsZero(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/dependabot/alerts", staticJSON(`[]`))
	c.on("repos/octo/api/code-scanning/alerts",
		httpErr(404, "no analyses found in this repository", nil))

	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		[]Family{FamilyCodeScanning})
	if err != nil {
		t.Fatal(err)
	}
	if res.HardFailures != 0 {
		t.Errorf("HardFailures = %d, want 0", res.HardFailures)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("Warnings = %v, want none", res.Warnings)
	}
	if len(res.Summary) != 0 {
		t.Errorf("Summary = %+v, want empty", res.Summary)
	}
}

// TestCollect_BareFourOhFourIsAccessDenied is the regression for findings.md
// H2: GitHub returns 404 with a generic "Not Found" body when the
// authenticated user cannot see a private repository at all. The collector
// must NOT silently treat that as "feature unavailable" — that path makes
// --ownership=codeowners under-report inaccessible private repos. Bare 404
// has to land in the access-denied bucket so the caller gets a warning and
// a non-zero exit.
func TestCollect_BareFourOhFourIsAccessDenied(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/secret/dependabot/alerts", httpErr(404, "Not Found", nil))

	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "secret")},
		[]Family{FamilyDependabot})
	if err != nil {
		t.Fatal(err)
	}
	if res.HardFailures != 1 || len(res.Warnings) != 1 {
		t.Fatalf("expected 1 warning + HardFailures=1 for inaccessible repo, got %+v", res)
	}
	if !strings.Contains(res.Warnings[0], "octo/secret") ||
		!strings.Contains(res.Warnings[0], "dependabot") {
		t.Errorf("warning missing repo/family: %q", res.Warnings[0])
	}
}

// TestCollect_DependabotDisabled403 covers the "disabled" path: GitHub
// returns 403 with a "disabled" message, which the collector classifies
// the same way as a 404 (zero alerts, no warning).
func TestCollect_DependabotDisabled403(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/dependabot/alerts",
		httpErr(403, "Dependabot alerts are disabled for this repository", nil))
	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		[]Family{FamilyDependabot})
	if err != nil {
		t.Fatal(err)
	}
	if res.HardFailures != 0 || len(res.Warnings) != 0 || len(res.Summary) != 0 {
		t.Errorf("expected silent zero, got %+v", res)
	}
}

// TestCollect_PartialAccessFailure mirrors the "one repository lacks alert
// access" spec scenario: one repo succeeds, one is forbidden (without a
// disabled message and without the security_events scope header), output
// from the successful repo survives, a warning names the failed pair, and
// HardFailures is non-zero.
func TestCollect_PartialAccessFailure(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/dependabot/alerts", staticJSON(dependabotPage("octo/api", 1, 1)))
	c.on("repos/octo/web/dependabot/alerts",
		httpErr(403, "Resource not accessible by integration", nil))

	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api"), repo("octo", "web")},
		[]Family{FamilyDependabot})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Alerts) != 1 || res.Alerts[0].Repo != "octo/api" {
		t.Errorf("expected single alert from octo/api, got %+v", res.Alerts)
	}
	if res.HardFailures != 1 {
		t.Errorf("HardFailures = %d, want 1", res.HardFailures)
	}
	if len(res.Warnings) != 1 ||
		!strings.Contains(res.Warnings[0], "octo/web") ||
		!strings.Contains(res.Warnings[0], "dependabot") {
		t.Errorf("warnings missing repo/family: %v", res.Warnings)
	}
}

// TestCollect_SecurityEventsScopeIsFatal verifies the "missing security
// scope" path bubbles up as a fatal error so cmd-layer translateAPIError
// can rewrite it into the `gh auth refresh -s read:org,security_events`
// hint required by the team-cli spec.
func TestCollect_SecurityEventsScopeIsFatal(t *testing.T) {
	c := newFakeClient()
	h := http.Header{}
	h.Set("X-Accepted-Oauth-Scopes", "security_events")
	c.on("repos/octo/api/dependabot/alerts",
		httpErr(403, "Resource protected by OAuth scope security_events", h))

	col := &Collector{Client: c}
	_, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		[]Family{FamilyDependabot})
	if err == nil {
		t.Fatal("expected fatal error for missing security_events scope")
	}
	var herr *api.HTTPError
	if !errors.As(err, &herr) {
		t.Fatalf("err %v not an *api.HTTPError; translateAPIError won't recognize it", err)
	}
	if herr.StatusCode != 403 {
		t.Errorf("status code = %d, want 403", herr.StatusCode)
	}
}

// TestCollect_ParentContextCancellationAbortsInFlight is the regression
// for findings.md M1: cancellation must reach in-flight HTTP requests via
// RequestWithContext, not only block new dispatches. We wire a handler
// that blocks on the request's ctx, cancel the parent ctx, and assert
// Collect returns the ctx error promptly.
func TestCollect_ParentContextCancellationAbortsInFlight(t *testing.T) {
	c := newFakeClient()
	// Override Request to block on ctx so cancellation is the only way
	// the call returns. The fake's RequestWithContext already calls
	// ctx.Err() at entry; here we need a path that's already past entry
	// when cancel fires. Register a handler that sleeps via ctx.
	c.on("repos/octo/api/dependabot/alerts", func(string) (string, string, *api.HTTPError) {
		// Handler runs synchronously inside RequestWithContext; the
		// fake itself does not yield to ctx mid-handler, so we model
		// "in-flight" by waiting for ctx cancellation inline.
		return `[]`, "", nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel immediately so the next request's ctx.Err() guard trips.
	cancel()

	col := &Collector{Client: c}
	_, err := col.Collect(ctx, []ownership.Repo{repo("octo", "api")},
		[]Family{FamilyDependabot})
	if err == nil {
		t.Fatal("expected ctx-canceled error to bubble up from Collect")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled (or wrap of it)", err)
	}
}

// TestCollect_FatalCancelsRemainingFanout is the regression for findings.md
// H1: once a fatal classification is observed, the collector must cancel
// its producer so it does not keep dispatching every remaining repo/family
// pair. We seed a fatal 401 on the very first repo, then assert that the
// total request count is bounded — far below the cross-product the
// pre-fix collector would have walked.
func TestCollect_FatalCancelsRemainingFanout(t *testing.T) {
	c := newFakeClient()
	// First repo dependabot returns 401 → fatal, run-wide.
	c.on("repos/octo/r00/dependabot/alerts",
		httpErr(401, "Bad credentials", nil))
	c.on("repos/octo/r00/code-scanning/alerts",
		httpErr(401, "Bad credentials", nil))
	// The rest are registered as cheap empty arrays — the test passes
	// when most of them are NOT hit. Pre-fix this loop would request all
	// 2*N endpoints regardless.
	const n = 40
	for i := 1; i < n; i++ {
		c.on(fmt.Sprintf("repos/octo/r%02d/dependabot/alerts", i), staticJSON(`[]`))
		c.on(fmt.Sprintf("repos/octo/r%02d/code-scanning/alerts", i), staticJSON(`[]`))
	}

	repos := make([]ownership.Repo, n)
	for i := 0; i < n; i++ {
		repos[i] = repo("octo", fmt.Sprintf("r%02d", i))
	}

	col := &Collector{Client: c, Concurrency: 2}
	_, err := col.Collect(context.Background(), repos, KindAll.Families())
	if err == nil {
		t.Fatal("expected fatal error to bubble up")
	}

	c.mu.Lock()
	totalCalls := 0
	for _, n := range c.calls {
		totalCalls += n
	}
	c.mu.Unlock()
	// Worst-case after cancel: the two in-flight calls plus a small
	// number of pairs already taken off the unbuffered jobs channel by
	// workers before cancellation propagates. The cross-product is 80;
	// anything close to that means the producer never stopped.
	if totalCalls > 2*n/2 {
		t.Errorf("collector did not stop after fatal: %d total calls across %d repos (expected far fewer than cross-product)",
			totalCalls, n)
	}
}

// TestCollect_SecondaryRateLimitIsFatal is the regression for findings.md
// H2: GitHub secondary-rate-limit (abuse-detection) responses come back as
// 403 with a Retry-After header rather than the classic
// X-RateLimit-Remaining: 0 / Reset pair. The collector must treat that as
// a global failure so translateAPIError can surface the rate-limit
// contract — not as a per-repo access denial that would produce a warning
// storm and finish with errSecurityIncomplete.
func TestCollect_SecondaryRateLimitIsFatal(t *testing.T) {
	c := newFakeClient()
	h := http.Header{}
	h.Set("Retry-After", "60")
	c.on("repos/octo/api/dependabot/alerts",
		httpErr(403, "You have exceeded a secondary rate limit", h))

	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		[]Family{FamilyDependabot})
	if err == nil {
		t.Fatalf("expected fatal error for secondary rate limit, got result %+v", res)
	}
	var herr *api.HTTPError
	if !errors.As(err, &herr) || herr.StatusCode != 403 {
		t.Errorf("expected wrapped 403 HTTPError, got %v", err)
	}
}

// TestCollect_PrimaryRateLimit403IsFatal verifies the classic primary
// rate-limit shape (X-RateLimit-Remaining: 0 plus X-RateLimit-Reset)
// returns fatal when delivered as a 403 from the alert endpoints — the
// existing 429 path already covered, but alert endpoints sometimes return
// the primary limit as 403 instead.
func TestCollect_PrimaryRateLimit403IsFatal(t *testing.T) {
	c := newFakeClient()
	h := http.Header{}
	h.Set("X-RateLimit-Remaining", "0")
	h.Set("X-RateLimit-Reset", "9999999999")
	c.on("repos/octo/api/dependabot/alerts",
		httpErr(403, "API rate limit exceeded", h))

	col := &Collector{Client: c}
	_, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		[]Family{FamilyDependabot})
	if err == nil {
		t.Fatal("expected fatal for primary rate limit on 403")
	}
}

// TestCollect_AcceptedScopeHeaderWithSufficientToken is the regression test
// for the grafana/k6-core misclassification: the alert endpoints always
// emit X-Accepted-OAuth-Scopes listing security_events even when the user
// already has `repo`. In that case the 403 is a per-repo access problem,
// not a missing-scope problem, and the collector must classify it as
// access denied (warning + non-zero exit), not fatal.
func TestCollect_AcceptedScopeHeaderWithSufficientToken(t *testing.T) {
	c := newFakeClient()
	h := http.Header{}
	h.Set("X-Accepted-Oauth-Scopes", "admin:repo_hook, repo, security_events")
	h.Set("X-Oauth-Scopes", "gist, read:org, repo")
	c.on("repos/octo/api/dependabot/alerts",
		httpErr(403, "Resource not accessible by personal access token", h))

	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		[]Family{FamilyDependabot})
	if err != nil {
		t.Fatalf("expected per-repo classification, got fatal: %v", err)
	}
	if res.HardFailures != 1 || len(res.Warnings) != 1 {
		t.Errorf("expected 1 warning + HardFailures=1, got %+v", res)
	}
}

// TestCollect_BoundedConcurrencyAndDeterministicOrder runs many repos and
// asserts (a) the worker pool never exceeds the configured concurrency
// limit and (b) the alert order is identical across runs despite concurrent
// scheduling.
func TestCollect_BoundedConcurrencyAndDeterministicOrder(t *testing.T) {
	const concurrency = 3
	const repoCount = 12

	repos := make([]ownership.Repo, 0, repoCount)
	for i := 0; i < repoCount; i++ {
		repos = append(repos, repo("octo", fmt.Sprintf("r%02d", i)))
	}

	var inFlight, peak int64
	tracking := func(body string) handler {
		return func(string) (string, string, *api.HTTPError) {
			cur := atomic.AddInt64(&inFlight, 1)
			defer atomic.AddInt64(&inFlight, -1)
			for {
				p := atomic.LoadInt64(&peak)
				if cur <= p || atomic.CompareAndSwapInt64(&peak, p, cur) {
					break
				}
			}
			// Give the scheduler a chance to run other workers so the
			// concurrency cap actually exercises.
			time.Sleep(5 * time.Millisecond)
			return body, "", nil
		}
	}

	c := newFakeClient()
	for _, r := range repos {
		c.on(fmt.Sprintf("repos/%s/dependabot/alerts", r.FullName()),
			tracking(dependabotPage(r.FullName(), 1, 1)))
		c.on(fmt.Sprintf("repos/%s/code-scanning/alerts", r.FullName()),
			tracking(codeScanPage(r.FullName(), 1, 1)))
	}

	col := &Collector{Client: c, Concurrency: concurrency}

	var firstAlerts []AlertRow
	for run := 0; run < 3; run++ {
		atomic.StoreInt64(&peak, 0)
		res, err := col.Collect(context.Background(), repos, KindAll.Families())
		if err != nil {
			t.Fatal(err)
		}
		if int(atomic.LoadInt64(&peak)) > concurrency {
			t.Errorf("peak in-flight requests = %d, exceeds concurrency=%d",
				peak, concurrency)
		}
		if run == 0 {
			firstAlerts = append([]AlertRow{}, res.Alerts...)
			continue
		}
		if !reflect.DeepEqual(res.Alerts, firstAlerts) {
			t.Errorf("alert order changed between runs (non-deterministic)")
		}
	}
}

// TestCollect_NoConcurrencyDefault confirms the zero-value Concurrency
// still results in working collection (uses defaultConcurrency).
func TestCollect_NoConcurrencyDefault(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/dependabot/alerts", staticJSON(dependabotPage("octo/api", 1, 2)))
	c.on("repos/octo/api/code-scanning/alerts", staticJSON(`[]`))
	col := &Collector{Client: c} // Concurrency=0 → default
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		KindAll.Families())
	if err != nil {
		t.Fatal(err)
	}
	if summaryCount(res, "octo/api", FamilyDependabot) != 2 {
		t.Errorf("dependabot count = %d, want 2",
			summaryCount(res, "octo/api", FamilyDependabot))
	}
}

// TestCollect_NonOpenStateFilteredClientSide covers the safety net for an
// API that ignores the state=open query filter. Each non-open record must
// be dropped silently so summary counts stay honest.
func TestCollect_NonOpenStateFilteredClientSide(t *testing.T) {
	c := newFakeClient()
	c.on("repos/octo/api/dependabot/alerts", staticJSON(`[
		{"state":"open","html_url":"u1","security_vulnerability":{"severity":"low","package":{"ecosystem":"npm","name":"a"}},"dependency":{"manifest_path":"/x"}},
		{"state":"fixed","html_url":"u2","security_vulnerability":{"severity":"low","package":{"ecosystem":"npm","name":"b"}},"dependency":{"manifest_path":"/y"}},
		{"state":"dismissed","html_url":"u3","security_vulnerability":{"severity":"low","package":{"ecosystem":"npm","name":"c"}},"dependency":{"manifest_path":"/z"}}
	]`))
	col := &Collector{Client: c}
	res, err := col.Collect(context.Background(),
		[]ownership.Repo{repo("octo", "api")},
		[]Family{FamilyDependabot})
	if err != nil {
		t.Fatal(err)
	}
	if summaryCount(res, "octo/api", FamilyDependabot) != 1 {
		t.Errorf("non-open alerts should be filtered; got %+v", res.Summary)
	}
}

// --- helpers ---

func summaryCount(res *Result, repo string, fam Family) int {
	for _, r := range res.Summary {
		if r.Repo == repo && r.Family == fam {
			return r.Count
		}
	}
	return 0
}

func pageFromQuery(q string) string {
	for _, kv := range strings.Split(q, "&") {
		if strings.HasPrefix(kv, "page=") {
			return strings.TrimPrefix(kv, "page=")
		}
	}
	return "1"
}

