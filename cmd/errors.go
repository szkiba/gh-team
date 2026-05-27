package cmd

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
)

// translateAPIError converts low-level GitHub HTTP errors into the
// user-facing messages required by the team-cli spec. It only inspects
// errors that wrap *api.HTTPError; anything else is passed through
// untouched, which keeps preflight's already-friendly "team does not
// exist" message intact.
func translateAPIError(err error) error {
	if err == nil {
		return nil
	}
	var herr *api.HTTPError
	if !errors.As(err, &herr) {
		return err
	}

	if herr.StatusCode == 401 {
		return errors.New("authentication failed: run `gh auth login` to sign in")
	}
	if herr.StatusCode == 403 || herr.StatusCode == 429 {
		if msg := rateLimitMessage(herr); msg != "" {
			return errors.New(msg)
		}
		if msg := missingScopeMessage(herr); msg != "" {
			return errors.New(msg)
		}
	}
	return err
}

// rateLimitMessage returns a user-facing string when the response looks
// like any flavor of GitHub rate-limit rejection. Returning "" means
// "not a rate-limit error" so the caller can fall through to other
// diagnostics.
//
// Three shapes are recognized:
//
//   - Primary limit: X-RateLimit-Remaining: 0 plus X-RateLimit-Reset.
//     The Resource header tells the user which bucket they hit (core,
//     graphql, search, code_search). Code search has a separate and
//     much lower limit than the REST core, so naming it explicitly
//     avoids the usual "but I barely made any requests" confusion.
//   - Secondary limit with Retry-After: GitHub's abuse-detection /
//     secondary-rate-limit responses come back as 403 with a
//     Retry-After header but without the primary-limit reset pair.
//     The actionable hint here is the retry interval.
//   - Secondary limit signaled only by message: some abuse-detection
//     responses populate neither set of headers but include a canonical
//     phrase in the body. Surface a generic secondary-limit message so
//     the user does not see a raw "HTTP 403" with no guidance.
func rateLimitMessage(herr *api.HTTPError) string {
	if herr.Headers.Get("X-RateLimit-Remaining") == "0" {
		reset := herr.Headers.Get("X-RateLimit-Reset")
		if reset != "" {
			if ts, err := strconv.ParseInt(reset, 10, 64); err == nil {
				resource := herr.Headers.Get("X-RateLimit-Resource")
				if resource == "" {
					resource = "core"
				}
				return fmt.Sprintf("GitHub %s rate limit exceeded; resets at %s UTC",
					resource, time.Unix(ts, 0).UTC().Format(time.RFC3339))
			}
		}
	}
	if retry := herr.Headers.Get("Retry-After"); retry != "" {
		// Retry-After is either delta-seconds or an HTTP-date per
		// RFC 9110. Synthesize an absolute UTC reset so the message
		// matches the contract documented in README and team-cli spec:
		// rate-limit errors always include the affected limit and an
		// absolute UTC reset time.
		if reset := parseRetryAfter(retry, time.Now()); !reset.IsZero() {
			return fmt.Sprintf("GitHub secondary rate limit hit; resets at %s UTC",
				reset.UTC().Format(time.RFC3339))
		}
		// Unparseable Retry-After: fall through to message-only path so
		// the user still sees secondary-limit guidance instead of a
		// raw HTTP error, even though we cannot quote a reset time.
	}
	msg := strings.ToLower(herr.Message)
	if strings.Contains(msg, "secondary rate limit") || strings.Contains(msg, "abuse detection") {
		return "GitHub secondary rate limit hit; reset time unavailable, wait a few minutes before retrying"
	}
	return ""
}

// parseRetryAfter converts a Retry-After header value into an absolute time
// instant. RFC 9110 §10.2.3 allows two forms:
//   - delta-seconds: a non-negative integer count of seconds.
//   - HTTP-date: an RFC 1123 / RFC 850 / asctime date.
//
// Returns the zero time on parse failure so the caller can decide whether
// to omit a reset time or fall back to a generic message.
func parseRetryAfter(value string, now time.Time) time.Time {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}
	}
	if secs, err := strconv.ParseInt(v, 10, 64); err == nil && secs >= 0 {
		return now.Add(time.Duration(secs) * time.Second)
	}
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, time.RFC850, time.ANSIC} {
		if t, err := time.Parse(layout, v); err == nil {
			return t
		}
	}
	return time.Time{}
}

// missingScopeMessage detects the "your token is fine but lacks the
// required scope" case. GitHub signals this with two response headers:
// X-Accepted-OAuth-Scopes lists the scopes the endpoint expects, and
// X-OAuth-Scopes lists what the token actually has. The reliable signal
// is an empty intersection between the two — the accepted header alone
// is not enough because GitHub returns it on every 403 from these
// endpoints, including pure repo-access failures.
//
// Security-alert endpoints accept either repo or security_events depending
// on visibility, but the maintainer baseline gh OAuth app does not include
// either by default — so when the accepted-scopes header names
// security_events, we recommend `read:org,security_events` so a single
// `gh auth refresh` covers both the team enumeration and the alert read.
func missingScopeMessage(herr *api.HTTPError) string {
	accepted := splitOAuthScopes(herr.Headers.Get("X-Accepted-Oauth-Scopes"))
	if len(accepted) == 0 {
		return ""
	}
	have := splitOAuthScopes(herr.Headers.Get("X-Oauth-Scopes"))
	if intersects(have, accepted) {
		return ""
	}
	for _, a := range accepted {
		if a == "security_events" {
			return "missing required OAuth scope for security-alert access: run `gh auth refresh -s read:org,security_events`"
		}
	}
	raw := herr.Headers.Get("X-Accepted-Oauth-Scopes")
	return fmt.Sprintf("missing required OAuth scope %q: run `gh auth refresh -s %s`", raw, raw)
}

// splitOAuthScopes normalizes a comma-separated OAuth-scopes header into a
// lowercase, trimmed slice. Returns nil for an empty header.
func splitOAuthScopes(header string) []string {
	if header == "" {
		return nil
	}
	parts := strings.Split(header, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		s := strings.ToLower(strings.TrimSpace(p))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func intersects(a, b []string) bool {
	for _, x := range a {
		for _, y := range b {
			if x == y {
				return true
			}
		}
	}
	return false
}
