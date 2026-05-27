package security

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/szkiba/gh-team/internal/ownership"
)

// pageSize is shared with ownership's collectors — 100 is the GitHub REST max
// for paginated list endpoints.
const pageSize = 100

// defaultConcurrency keeps repository/family fanout bounded so multi-repo
// runs are materially faster than sequential but do not burst rate limits.
// Surfaced as a constant rather than a flag per design.md Decision 7.
const defaultConcurrency = 4

// Client is the narrow subset of *api.RESTClient the collector needs. We use
// RequestWithContext rather than Get because:
//
//  1. The Dependabot and code-scanning alert endpoints use Link-header
//     cursor pagination — the `page=` query parameter is rejected with
//     HTTP 400 — so callers must read the Link header to find the next
//     URL. Request returns the raw *http.Response, which gives us both
//     the header and the body.
//  2. The context-aware variant lets us cancel in-flight HTTP requests
//     when the parent ctx is canceled (Ctrl-C, parent command cancel,
//     or our own internal cancel on first fatal classification). Bare
//     Request() uses context.Background() inside go-gh and would leak
//     the request through cancellation.
//
// *api.RESTClient.RequestWithContext returns *api.HTTPError for non-2xx
// responses, matching the error shape classifyErr already understands.
type Client interface {
	RequestWithContext(ctx context.Context, method, path string, body io.Reader) (*http.Response, error)
}

// Collector orchestrates repo/family alert collection with bounded
// concurrency. Concurrency must be > 0; zero falls back to defaultConcurrency.
type Collector struct {
	Client      Client
	Concurrency int
}

// pairResult is the bounded-channel payload emitted by each worker.
type pairResult struct {
	repo    ownership.Repo
	family  Family
	alerts  []AlertRow
	status  pairStatus
	message string
	// fatalErr preserves the original error so translateAPIError can
	// still detect 401/rate-limit/scope cases when the collector bubbles
	// a hard failure to the caller.
	fatalErr error
}

type pairStatus int

const (
	statusOK pairStatus = iota
	// statusUnavailable: feature not enabled / no analyses configured —
	// contributes zero alerts and no warning.
	statusUnavailable
	// statusAccessDenied: caller lacks scope or repo-level permission for
	// the family — contributes zero alerts plus stderr warning and forces
	// a non-zero final exit.
	statusAccessDenied
	// statusFatal: error the collector cannot continue past (auth missing,
	// rate-limit, generic 5xx). Returned to the caller as-is.
	statusFatal
)

// Collect requests the cross-product of repos × families with bounded
// concurrency. The slice ordering of `repos` and `families` does not affect
// output ordering; sortSummary/sortAlerts guarantee determinism.
func (c *Collector) Collect(ctx context.Context, repos []ownership.Repo, families []Family) (*Result, error) {
	conc := c.Concurrency
	if conc <= 0 {
		conc = defaultConcurrency
	}

	// Local cancelable child ctx so the result loop can stop the producer
	// the moment the first fatal error is observed. Without this, a global
	// failure (missing scope, rate-limit) still fans out into every
	// remaining repo/family pair before the command returns — see
	// findings.md H1.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan job)
	results := make(chan pairResult)

	var workersWG sync.WaitGroup
	for i := 0; i < conc; i++ {
		workersWG.Add(1)
		go func() {
			defer workersWG.Done()
			for j := range jobs {
				results <- runPair(ctx, c.Client, j)
			}
		}()
	}

	// Producer enqueues the repo × family cross-product. Once ctx is
	// canceled (either by the parent or by a fatal classification in the
	// result loop below), the select exits and close(jobs) unblocks the
	// workers' `for j := range jobs` loops, so in-flight requests finish
	// but no new ones are dispatched.
	go func() {
		defer close(jobs)
		for _, r := range repos {
			for _, f := range families {
				select {
				case <-ctx.Done():
					return
				case jobs <- job{repo: r, family: f}:
				}
			}
		}
	}()

	go func() {
		workersWG.Wait()
		close(results)
	}()

	res := &Result{}
	counts := map[string]int{} // key: "<repo>\t<family>"

	var firstFatal error
	for pr := range results {
		switch pr.status {
		case statusFatal:
			if firstFatal == nil {
				if pr.fatalErr != nil {
					firstFatal = pr.fatalErr
				} else {
					firstFatal = errors.New(pr.message)
				}
			}
			// Cancel the producer so we stop dispatching new pairs
			// after the first global failure. Workers still draining
			// in-flight requests will arrive here and be discarded.
			cancel()
			continue
		case statusAccessDenied:
			res.Warnings = append(res.Warnings,
				fmt.Sprintf("warning: cannot read %s alerts for %s: %s",
					pr.family, pr.repo.FullName(), pr.message))
			res.HardFailures++
			continue
		case statusUnavailable:
			continue
		}
		res.Alerts = append(res.Alerts, pr.alerts...)
		key := pr.repo.FullName() + "\t" + string(pr.family)
		counts[key] += len(pr.alerts)
	}

	if firstFatal != nil {
		return nil, firstFatal
	}
	// Surface parent-context cancellation even when no worker observed
	// it as a fatal HTTPError — otherwise an immediately-canceled run
	// would silently return an empty result.
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	for key, n := range counts {
		if n == 0 {
			continue
		}
		parts := strings.SplitN(key, "\t", 2)
		res.Summary = append(res.Summary, SummaryRow{
			Repo:   parts[0],
			Family: Family(parts[1]),
			Count:  n,
		})
	}
	sortSummary(res.Summary)
	sortAlerts(res.Alerts)
	sort.Strings(res.Warnings)
	return res, nil
}

type job struct {
	repo   ownership.Repo
	family Family
}

// runPair queries one repository/family pair, paginates fully, and maps any
// REST error onto one of the pairStatus values per spec error semantics.
// The ctx is propagated all the way down to the HTTP transport so a
// canceled parent context aborts in-flight requests, not just future ones.
func runPair(ctx context.Context, c Client, j job) pairResult {
	var (
		alerts []AlertRow
		err    error
	)
	switch j.family {
	case FamilyDependabot:
		alerts, err = fetchDependabot(ctx, c, j.repo.Owner, j.repo.Name)
	case FamilyCodeScanning:
		alerts, err = fetchCodeScanning(ctx, c, j.repo.Owner, j.repo.Name)
	default:
		return pairResult{repo: j.repo, family: j.family, status: statusFatal,
			message: fmt.Sprintf("unknown alert family %q", j.family)}
	}
	if err != nil {
		return classifyErr(j, err)
	}
	return pairResult{repo: j.repo, family: j.family, alerts: alerts, status: statusOK}
}

// classifyErr maps the per-pair HTTP error into one of: unavailable, access
// denied, or fatal. The rules:
//   - 404 with a feature-disabled message ("no analyses found", "not
//     enabled", etc.): treat as feature unavailable → zero alerts, no
//     warning.
//   - 404 without such a message: treat as repository inaccessible →
//     warning + exit 1. For private resources GitHub returns 404 for
//     "you cannot access this repository", so a bare 404 must never
//     silently drop the pair, otherwise --ownership=codeowners can
//     under-report private repos the caller cannot read.
//   - 403 with a disabled/not-enabled message in the body: feature
//     unavailable, same as the 404-disabled case.
//   - 403 otherwise: access denied → warning + exit 1.
//   - 401, rate-limit, or anything else with a non-nil HTTPError: fatal,
//     bubble up so the existing translateAPIError gives the user
//     `gh auth login` / rate-limit guidance.
//   - non-HTTPError: fatal.
func classifyErr(j job, err error) pairResult {
	var herr *api.HTTPError
	if !errors.As(err, &herr) {
		return pairResult{repo: j.repo, family: j.family, status: statusFatal, message: err.Error(), fatalErr: err}
	}
	switch herr.StatusCode {
	case 404:
		if isDisabledMessage(herr.Message) {
			return pairResult{repo: j.repo, family: j.family, status: statusUnavailable}
		}
		return pairResult{repo: j.repo, family: j.family,
			status: statusAccessDenied, message: shortMsg(herr)}
	case 403:
		// A 403 here can mean four different things:
		//   1. Primary or secondary rate-limit / abuse-detection — fatal,
		//      run-wide; translateAPIError already knows how to format
		//      the absolute reset time, but secondary limits do not
		//      always set X-RateLimit-Remaining: 0, so we also accept
		//      Retry-After and the canonical message phrases.
		//   2. The feature is disabled for this repo — silent zero.
		//   3. The user's OAuth token has none of the scopes the endpoint
		//      accepts — fatal, run-wide scope error.
		//   4. The token has a sufficient scope but the user lacks
		//      repo-level access — per-repo warning + non-zero exit.
		//
		// X-Accepted-OAuth-Scopes always lists every scope that GitHub
		// would accept for the endpoint (e.g. `repo, security_events,
		// ...`), so its mere presence is not a signal of scope failure.
		// The reliable signal is the *intersection* with X-OAuth-Scopes:
		// when none of the token's scopes appear in the accepted list,
		// the token genuinely cannot reach this endpoint and the user
		// needs to refresh.
		if isRateLimitHTTPError(herr) {
			return pairResult{repo: j.repo, family: j.family, status: statusFatal,
				message: err.Error(), fatalErr: err}
		}
		if isDisabledMessage(herr.Message) {
			return pairResult{repo: j.repo, family: j.family, status: statusUnavailable}
		}
		if tokenLacksAllAcceptedScopes(herr) {
			return pairResult{repo: j.repo, family: j.family, status: statusFatal,
				message: err.Error(), fatalErr: err}
		}
		return pairResult{repo: j.repo, family: j.family,
			status: statusAccessDenied, message: shortMsg(herr)}
	}
	return pairResult{repo: j.repo, family: j.family, status: statusFatal, message: err.Error(), fatalErr: err}
}

// isRateLimitHTTPError returns true when a 403/429 response actually
// represents a primary or secondary GitHub rate-limit, not a permission
// failure. The collector needs this so secondary-rate-limit 403s do not
// degrade into a warning storm of per-repo access-denied messages and
// instead bubble up to translateAPIError's rate-limit handling — see
// findings.md H2.
//
// Primary rate-limits set X-RateLimit-Remaining: 0 plus X-RateLimit-Reset
// and are already covered by cmd/errors.go's translation. Secondary
// (abuse-detection) rate-limits do not always populate those headers,
// but they consistently include a Retry-After header and/or one of the
// canonical message strings GitHub uses for that family of failures.
func isRateLimitHTTPError(herr *api.HTTPError) bool {
	if herr.Headers.Get("X-RateLimit-Remaining") == "0" &&
		herr.Headers.Get("X-RateLimit-Reset") != "" {
		return true
	}
	if herr.Headers.Get("Retry-After") != "" {
		return true
	}
	m := strings.ToLower(herr.Message)
	return strings.Contains(m, "secondary rate limit") ||
		strings.Contains(m, "abuse detection") ||
		strings.Contains(m, "api rate limit exceeded")
}

// tokenLacksAllAcceptedScopes returns true when the token's scopes
// (X-OAuth-Scopes) and the endpoint's accepted scopes
// (X-Accepted-OAuth-Scopes) share no element. That is the only reliable
// "missing scope" signal GitHub gives us: the accepted-scopes header is
// always populated on these endpoints even when the caller's scope is
// fine and the 403 is really a repo-level access problem.
//
// Both headers are comma-separated lists; entries may include surrounding
// whitespace. Comparison is case-insensitive to match GitHub's behavior.
func tokenLacksAllAcceptedScopes(herr *api.HTTPError) bool {
	accepted := splitScopes(herr.Headers.Get("X-Accepted-Oauth-Scopes"))
	if len(accepted) == 0 {
		return false
	}
	have := splitScopes(herr.Headers.Get("X-Oauth-Scopes"))
	if len(have) == 0 {
		return true
	}
	for _, h := range have {
		for _, a := range accepted {
			if h == a {
				return false
			}
		}
	}
	return true
}

// splitScopes normalizes a comma-separated OAuth-scopes header into a
// lowercase, trimmed slice. Returns nil for an empty header.
func splitScopes(header string) []string {
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

// isDisabledMessage looks for the GitHub phrases that signal a feature is
// off rather than blocked by permissions. Substring match is intentional:
// GitHub varies these strings across endpoints and over time, and false
// negatives just degrade to a per-repo warning rather than silent loss.
func isDisabledMessage(msg string) bool {
	m := strings.ToLower(msg)
	return strings.Contains(m, "disabled") ||
		strings.Contains(m, "not enabled") ||
		strings.Contains(m, "must be enabled") ||
		strings.Contains(m, "no analysis found") ||
		strings.Contains(m, "no analyses found")
}

func shortMsg(herr *api.HTTPError) string {
	if herr.Message != "" {
		return herr.Message
	}
	return fmt.Sprintf("HTTP %d", herr.StatusCode)
}
