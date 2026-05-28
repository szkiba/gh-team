package cmd

import (
	"net/http"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

// TestMissingScopeMessage_RepoScopeRemediation covers the team-cli spec
// scenario "Missing repository-read scope for security prs on private
// repos": a 403 whose accepted-scopes header contains `repo` (and the
// token's scopes do not intersect) must surface the targeted
// `gh auth refresh -s repo` guidance, NOT the security_events guidance
// nor the generic fallback.
func TestMissingScopeMessage_RepoScopeRemediation(t *testing.T) {
	h := http.Header{}
	h.Set("X-Accepted-Oauth-Scopes", "repo, public_repo")
	h.Set("X-Oauth-Scopes", "read:org")
	herr := &api.HTTPError{StatusCode: 403, Message: "Resource not accessible", Headers: h}

	got := translateAPIError(herr).Error()
	if !strings.Contains(got, "pull requests") {
		t.Errorf("expected pull-request remediation, got %q", got)
	}
	if !strings.Contains(got, "gh auth refresh -s repo") {
		t.Errorf("expected `gh auth refresh -s repo` in %q", got)
	}
	if strings.Contains(got, "security_events") {
		t.Errorf("must NOT recommend security_events for pulls-endpoint scope failure: %q", got)
	}
}

// TestMissingScopeMessage_SecurityEventsRemediationUnchanged guards the
// pre-existing security_events branch from regression now that a repo-scope
// branch has been added below it.
func TestMissingScopeMessage_SecurityEventsRemediationUnchanged(t *testing.T) {
	h := http.Header{}
	h.Set("X-Accepted-Oauth-Scopes", "repo, security_events")
	h.Set("X-Oauth-Scopes", "read:org")
	herr := &api.HTTPError{StatusCode: 403, Message: "Resource not accessible", Headers: h}

	got := translateAPIError(herr).Error()
	if !strings.Contains(got, "security_events") {
		t.Errorf("expected security_events remediation, got %q", got)
	}
	if !strings.Contains(got, "gh auth refresh -s read:org,security_events") {
		t.Errorf("expected refresh command in %q", got)
	}
}

// TestMissingScopeMessage_TokenIntersectsAccepted: when the token's scopes
// overlap with accepted scopes the 403 is NOT a scope failure (it's a
// repo-level access failure) and missingScopeMessage must return "".
func TestMissingScopeMessage_TokenIntersectsAccepted(t *testing.T) {
	h := http.Header{}
	h.Set("X-Accepted-Oauth-Scopes", "repo")
	h.Set("X-Oauth-Scopes", "repo, read:org")
	herr := &api.HTTPError{StatusCode: 403, Message: "Resource not accessible", Headers: h}

	got := translateAPIError(herr)
	// Should pass through unchanged because no missing-scope or
	// rate-limit pattern matched.
	if got == nil {
		t.Fatal("expected error pass-through, got nil")
	}
	if _, ok := got.(*api.HTTPError); !ok {
		t.Errorf("expected raw *api.HTTPError pass-through, got %T", got)
	}
}
