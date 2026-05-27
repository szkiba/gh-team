package cmd

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// TestSecuritySummary_RejectsInvalidKind covers the spec's "unsupported alert
// family rejected" scenario: --kind=secret-scanning must surface an error
// whose message names every supported value so the user can fix it.
func TestSecuritySummary_RejectsInvalidKind(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"security", "summary", "octo/platform", "--kind=secret-scanning"})

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --kind")
	}
	msg := err.Error()
	for _, want := range []string{"dependabot", "code-scanning", "all"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q missing supported value %q", msg, want)
		}
	}
}

func TestSecurityAlerts_RejectsInvalidKind(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"security", "alerts", "octo/platform", "--kind=secret-scanning"})

	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for invalid --kind")
	}
}

// TestSecurityAlerts_RejectsBadTeamArg verifies the shared parseOrgTeam
// validation still runs before any REST call is attempted, so a malformed
// argument fails fast.
func TestSecurityAlerts_RejectsBadTeamArg(t *testing.T) {
	root := newRootCmd()
	root.SetArgs([]string{"security", "alerts", "not-a-team"})
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "expected <org>/<team-slug>") {
		t.Errorf("expected parseOrgTeam error, got %v", err)
	}
}

// TestTranslateAPIError_SecurityEventsScope verifies that the security_events
// path produces the exact `gh auth refresh -s read:org,security_events`
// wording required by the team-cli spec scenario "Missing security-events
// scope". The HTTP-error fixture mimics what GitHub returns for the alert
// endpoints when the token lacks the scope.
func TestTranslateAPIError_SecurityEventsScope(t *testing.T) {
	h := http.Header{}
	h.Set("X-Accepted-Oauth-Scopes", "security_events")
	herr := &api.HTTPError{
		StatusCode: 403,
		Message:    "Resource not accessible by integration",
		Headers:    h,
	}
	out := translateAPIError(herr)
	if out == nil {
		t.Fatal("expected non-nil error")
	}
	msg := out.Error()
	for _, want := range []string{"security_events", "gh auth refresh", "read:org"} {
		if !strings.Contains(msg, want) {
			t.Errorf("translated error %q missing %q", msg, want)
		}
	}
}

// TestTranslateAPIError_OrgScopeUnchanged guards against the security
// branch overriding the existing org-scope path: a 403 with only `read:org`
// in the accepted scopes header must still produce the generic
// `gh auth refresh -s read:org` hint.
func TestTranslateAPIError_OrgScopeUnchanged(t *testing.T) {
	h := http.Header{}
	h.Set("X-Accepted-Oauth-Scopes", "read:org")
	herr := &api.HTTPError{
		StatusCode: 403,
		Message:    "Resource not accessible",
		Headers:    h,
	}
	out := translateAPIError(herr)
	msg := out.Error()
	if !strings.Contains(msg, "read:org") {
		t.Errorf("translated %q missing read:org guidance", msg)
	}
	if strings.Contains(msg, "security_events") {
		t.Errorf("translated %q should not mention security_events for non-security scope", msg)
	}
}

// TestTranslateAPIError_TokenHasAcceptedScopeIsPassthrough is the regression
// for the bug we hit against grafana/k6-core: a 403 from a per-repo access
// problem still includes security_events in X-Accepted-OAuth-Scopes even
// when the token already holds `repo`. The translator must NOT rewrite
// such 403s into a misleading scope hint — it should fall through and
// preserve the original error so the per-repo warning path runs.
func TestTranslateAPIError_TokenHasAcceptedScopeIsPassthrough(t *testing.T) {
	h := http.Header{}
	h.Set("X-Accepted-Oauth-Scopes", "admin:repo_hook, repo, security_events")
	h.Set("X-Oauth-Scopes", "gist, read:org, repo")
	herr := &api.HTTPError{
		StatusCode: 403,
		Message:    "Resource not accessible by personal access token",
		Headers:    h,
	}
	out := translateAPIError(herr)
	if out == nil {
		t.Fatal("expected non-nil error")
	}
	msg := out.Error()
	if strings.Contains(msg, "security_events") {
		t.Errorf("translated %q must not surface a scope hint when token already has an accepted scope", msg)
	}
	if strings.Contains(msg, "missing required OAuth scope") {
		t.Errorf("translated %q must not claim missing scope when intersection is non-empty", msg)
	}
}

// TestTranslateAPIError_SecondaryRateLimitRetryAfterDeltaSeconds is the
// regression for findings.md H1: secondary-rate-limit responses arrive as
// 403 with a Retry-After header in delta-seconds form. The translator must
// honor the documented contract by quoting an absolute UTC reset time
// derived from now+seconds, not a raw "retry after N seconds" hint.
func TestTranslateAPIError_SecondaryRateLimitRetryAfterDeltaSeconds(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("Retry-After", "60")
	herr := &api.HTTPError{
		StatusCode: 403,
		Message:    "You have exceeded a secondary rate limit",
		Headers:    hdr,
	}
	out := translateAPIError(herr)
	if out == nil {
		t.Fatal("expected non-nil error")
	}
	msg := out.Error()
	if !strings.Contains(msg, "secondary rate limit") {
		t.Errorf("translated %q missing 'secondary rate limit'", msg)
	}
	if !strings.Contains(msg, "resets at") {
		t.Errorf("translated %q missing 'resets at' (contract requires absolute reset)", msg)
	}
	if !strings.Contains(msg, "UTC") {
		t.Errorf("translated %q missing UTC suffix", msg)
	}
}

// TestTranslateAPIError_SecondaryRateLimitRetryAfterHTTPDate verifies the
// HTTP-date variant of Retry-After per RFC 9110 — the value is parsed
// directly into an absolute time without depending on now().
func TestTranslateAPIError_SecondaryRateLimitRetryAfterHTTPDate(t *testing.T) {
	hdr := http.Header{}
	hdr.Set("Retry-After", "Wed, 27 May 2026 12:34:56 GMT")
	herr := &api.HTTPError{
		StatusCode: 403,
		Message:    "You have exceeded a secondary rate limit",
		Headers:    hdr,
	}
	out := translateAPIError(herr)
	if out == nil {
		t.Fatal("expected non-nil error")
	}
	msg := out.Error()
	if !strings.Contains(msg, "2026-05-27T12:34:56Z") {
		t.Errorf("translated %q missing parsed HTTP-date as RFC3339 UTC", msg)
	}
}

// TestParseRetryAfter exercises the two RFC 9110 forms plus failure paths.
func TestParseRetryAfter(t *testing.T) {
	now := time.Date(2026, 5, 27, 6, 0, 0, 0, time.UTC)
	cases := []struct {
		name, in   string
		wantZero   bool
		wantString string // RFC3339 UTC if !wantZero
	}{
		{"delta-seconds", "120", false, "2026-05-27T06:02:00Z"},
		{"zero-delta", "0", false, "2026-05-27T06:00:00Z"},
		{"http-date-RFC1123", "Wed, 27 May 2026 12:34:56 GMT", false, "2026-05-27T12:34:56Z"},
		{"empty", "", true, ""},
		{"garbage", "soon", true, ""},
		{"negative", "-5", true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseRetryAfter(tc.in, now)
			if tc.wantZero {
				if !got.IsZero() {
					t.Errorf("parseRetryAfter(%q) = %v, want zero", tc.in, got)
				}
				return
			}
			if got.IsZero() {
				t.Fatalf("parseRetryAfter(%q) = zero, want %s", tc.in, tc.wantString)
			}
			if g := got.UTC().Format(time.RFC3339); g != tc.wantString {
				t.Errorf("parseRetryAfter(%q) = %s, want %s", tc.in, g, tc.wantString)
			}
		})
	}
}

// TestTranslateAPIError_SecondaryRateLimitMessageOnly covers the variant
// where neither Retry-After nor primary-limit headers are populated, but
// the body's canonical phrase identifies the failure. Reset time cannot
// be synthesized here; the spec/README contract is narrowed to allow this
// case to surface a "reset time unavailable" qualifier rather than a raw
// HTTP error.
func TestTranslateAPIError_SecondaryRateLimitMessageOnly(t *testing.T) {
	herr := &api.HTTPError{
		StatusCode: 403,
		Message:    "You have triggered an abuse detection mechanism",
		Headers:    http.Header{},
	}
	out := translateAPIError(herr)
	if out == nil {
		t.Fatal("expected non-nil error")
	}
	msg := out.Error()
	if !strings.Contains(msg, "secondary rate limit") {
		t.Errorf("translated %q missing 'secondary rate limit'", msg)
	}
	if !strings.Contains(msg, "reset time unavailable") {
		t.Errorf("translated %q must qualify missing reset (contract narrowing)", msg)
	}
}

// TestErrSecurityIncomplete_IsTypedNotWrapped guards that the exit-status
// error is its own type so callers (or future code) can detect it without
// brittle string matching.
func TestErrSecurityIncomplete_IsTypedNotWrapped(t *testing.T) {
	var e errSecurityIncomplete
	if !errors.As(errSecurityIncomplete{count: 1}, &e) {
		t.Fatal("errSecurityIncomplete should be detectable via errors.As")
	}
}
