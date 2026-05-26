package cmd

import (
	"errors"
	"fmt"
	"strconv"
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
// like a rate-limit rejection (X-RateLimit-Remaining: 0 plus a reset
// header). Returning "" means "not a rate-limit error" so the caller
// can fall through to other diagnostics.
//
// The Resource header tells the user which bucket they hit (core,
// graphql, search, code_search). Code search has a separate and much
// lower limit than the REST core, so naming it explicitly avoids the
// usual "but I barely made any requests" confusion.
func rateLimitMessage(herr *api.HTTPError) string {
	if herr.Headers.Get("X-RateLimit-Remaining") != "0" {
		return ""
	}
	reset := herr.Headers.Get("X-RateLimit-Reset")
	if reset == "" {
		return ""
	}
	ts, err := strconv.ParseInt(reset, 10, 64)
	if err != nil {
		return ""
	}
	resource := herr.Headers.Get("X-RateLimit-Resource")
	if resource == "" {
		resource = "core"
	}
	return fmt.Sprintf("GitHub %s rate limit exceeded; resets at %s UTC",
		resource, time.Unix(ts, 0).UTC().Format(time.RFC3339))
}

// missingScopeMessage detects the "your token is fine but lacks the
// required scope" case. GitHub signals this via the
// X-Accepted-OAuth-Scopes response header: it lists the scopes the
// endpoint expects, which we surface verbatim in the `gh auth refresh`
// hint so the user can copy-paste it.
func missingScopeMessage(herr *api.HTTPError) string {
	accepted := herr.Headers.Get("X-Accepted-Oauth-Scopes")
	if accepted == "" {
		return ""
	}
	return fmt.Sprintf("missing required OAuth scope %q: run `gh auth refresh -s %s`", accepted, accepted)
}
