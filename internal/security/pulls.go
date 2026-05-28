package security

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"

	"github.com/szkiba/gh-team/internal/ownership"
)

// DefaultPullTitleRegex is the v1 default `--title` pattern. The three
// anchored alternatives cover the common security PR conventions:
//   - `[security] ...`
//   - `security: ...`
//   - `... [security]`
//
// Case-insensitive flag handles `[Security]` / `[SECURITY]`.
func DefaultPullTitleRegex() *regexp.Regexp {
	return regexp.MustCompile(`(?i)^\[security\]|^security:|\[security\]$`)
}

// DefaultPullLabel is the v1 default `--label` value.
const DefaultPullLabel = "security"

// PullRequest is one row of `security prs` output. Field names match the
// JSON / template field-name contract.
type PullRequest struct {
	Repo    string
	Number  int
	State   string
	Title   string
	Author  string
	Updated string
	URL     string
}

// PullMatcher decides whether a PR matches at least one configured security
// signal. Title and Labels are OR-combined: a PR matches when Title matches
// the regex OR when any of its labels exactly equals one of the configured
// labels.
//
// A nil Title means "no title signal"; an empty Labels slice means "no label
// signal". Both nil/empty produces a matcher that matches nothing.
type PullMatcher struct {
	Title  *regexp.Regexp
	Labels []string
}

// Match returns true when the PR's title or any of its labels satisfies the
// configured signals.
func (m *PullMatcher) Match(title string, labels []string) bool {
	if m == nil {
		return false
	}
	if m.Title != nil && m.Title.MatchString(title) {
		return true
	}
	for _, want := range m.Labels {
		for _, got := range labels {
			if got == want {
				return true
			}
		}
	}
	return false
}

// PullsResult is what PullsCollector.Collect returns. PullRequests are
// already sorted (repo asc, number desc). Warnings are stderr lines for the
// caller to emit. HardFailures > 0 means the caller must exit non-zero
// after rendering.
type PullsResult struct {
	PullRequests []PullRequest
	Warnings     []string
	HardFailures int
}

// PullsCollector orchestrates per-repository pull-request collection with
// bounded concurrency. The matcher is applied client-side, so a PR is
// dropped before reaching the result if it fails to match.
type PullsCollector struct {
	Client      Client
	Concurrency int
	Matcher     *PullMatcher
}

type pullsJob struct {
	repo ownership.Repo
}

type pullsResult struct {
	repo     ownership.Repo
	prs      []PullRequest
	status   pairStatus
	message  string
	fatalErr error
}

// Collect dispatches one fetchPulls per repository with bounded concurrency
// and merges the matching rows into a deterministically sorted result. The
// classify rules mirror the alert collector: 403/404 with rate-limit or
// scope semantics are fatal; 403/404 with repository-level access denial
// are per-repo warnings.
func (c *PullsCollector) Collect(ctx context.Context, repos []ownership.Repo) (*PullsResult, error) {
	conc := c.Concurrency
	if conc <= 0 {
		conc = defaultConcurrency
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan pullsJob)
	results := make(chan pullsResult)

	var workersWG sync.WaitGroup
	for i := 0; i < conc; i++ {
		workersWG.Add(1)
		go func() {
			defer workersWG.Done()
			for j := range jobs {
				results <- c.runOne(ctx, j)
			}
		}()
	}

	go func() {
		defer close(jobs)
		for _, r := range repos {
			select {
			case <-ctx.Done():
				return
			case jobs <- pullsJob{repo: r}:
			}
		}
	}()

	go func() {
		workersWG.Wait()
		close(results)
	}()

	res := &PullsResult{}
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
			cancel()
			continue
		case statusAccessDenied:
			res.Warnings = append(res.Warnings,
				fmt.Sprintf("warning: cannot read pull requests for %s: %s",
					pr.repo.FullName(), pr.message))
			res.HardFailures++
			continue
		case statusUnavailable:
			continue
		}
		res.PullRequests = append(res.PullRequests, pr.prs...)
	}

	if firstFatal != nil {
		return nil, firstFatal
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	sortPullRequests(res.PullRequests)
	sort.Strings(res.Warnings)
	return res, nil
}

func (c *PullsCollector) runOne(ctx context.Context, j pullsJob) pullsResult {
	prs, err := fetchPulls(ctx, c.Client, j.repo.Owner, j.repo.Name, c.Matcher)
	if err != nil {
		return classifyPullsErr(j.repo, err)
	}
	return pullsResult{repo: j.repo, prs: prs, status: statusOK}
}

// sortPullRequests applies the documented "repo asc, number desc" ordering.
// PR numbers are unique within a repository so the sort is total.
func sortPullRequests(rows []PullRequest) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Repo != rows[j].Repo {
			return rows[i].Repo < rows[j].Repo
		}
		return rows[i].Number > rows[j].Number
	})
}

// classifyPullsErr maps a per-repo HTTP error onto a pairStatus. Unlike
// the alert collector, scope-missing 403s on `pulls` are not run-wide:
// public repositories list without `repo` scope, so a missing scope
// only affects private repos. Cancelling the whole run on the first
// such 403 would discard public-repo rows the user is entitled to.
// Each scope-missing repo is reported per-repo with the targeted
// remediation inlined in the warning text.
//
//   - 404: repository inaccessible — per-repo warning, exit 1.
//   - 403 rate-limit: fatal, run-wide.
//   - 403 with no overlap between accepted scopes and the token's scopes:
//     per-repo warning naming the missing scope and the
//     `gh auth refresh -s repo` remediation; exit 1.
//   - 403 otherwise: per-repo access denied — warning, exit 1.
//   - any other non-HTTPError or non-2xx: fatal.
func classifyPullsErr(repo ownership.Repo, err error) pullsResult {
	var herr *api.HTTPError
	if !errors.As(err, &herr) {
		return pullsResult{repo: repo, status: statusFatal, message: err.Error(), fatalErr: err}
	}
	switch herr.StatusCode {
	case 404:
		return pullsResult{repo: repo, status: statusAccessDenied, message: shortMsg(herr)}
	case 403:
		if isRateLimitHTTPError(herr) {
			return pullsResult{repo: repo, status: statusFatal, message: err.Error(), fatalErr: err}
		}
		if tokenLacksAllAcceptedScopes(herr) {
			return pullsResult{repo: repo, status: statusAccessDenied,
				message: "missing repository-read OAuth scope; run `gh auth refresh -s repo` (or grant a fine-grained `Pull requests: read` permission)"}
		}
		return pullsResult{repo: repo, status: statusAccessDenied, message: shortMsg(herr)}
	}
	return pullsResult{repo: repo, status: statusFatal, message: err.Error(), fatalErr: err}
}

// prPayload is the per-PR JSON projection. Only fields needed for matching
// and the public field-name contract are deserialized.
type prPayload struct {
	Number    int    `json:"number"`
	State     string `json:"state"`
	Title     string `json:"title"`
	HTMLURL   string `json:"html_url"`
	UpdatedAt string `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func pullsInitialPath(owner, repo string) string {
	return fmt.Sprintf("repos/%s/%s/pulls?state=open&per_page=%d", owner, repo, pageSize)
}

// fetchPulls walks the Link-header pagination for one repository's open
// pull requests, applies the matcher, and projects matches into
// PullRequest rows. The matcher runs client-side because GitHub's pulls
// endpoint has no built-in title-regex or label-OR filter.
func fetchPulls(ctx context.Context, c Client, owner, repo string, matcher *PullMatcher) ([]PullRequest, error) {
	full := owner + "/" + repo
	var out []PullRequest
	err := paginate(ctx, c, pullsInitialPath(owner, repo), func(body []byte) error {
		var batch []prPayload
		if err := decodeJSONArray(body, &batch); err != nil {
			return err
		}
		for _, p := range batch {
			if p.State != "" && p.State != "open" {
				continue
			}
			labels := make([]string, 0, len(p.Labels))
			for _, l := range p.Labels {
				labels = append(labels, l.Name)
			}
			if !matcher.Match(p.Title, labels) {
				continue
			}
			out = append(out, PullRequest{
				Repo:    full,
				Number:  p.Number,
				State:   p.State,
				Title:   p.Title,
				Author:  p.User.Login,
				Updated: normalizeUpdated(p.UpdatedAt),
				URL:     p.HTMLURL,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// normalizeUpdated renders the API's RFC 3339 timestamp in fixed UTC with
// the trailing `Z` so downstream consumers see a single deterministic
// format. Empty input or a parse miss falls through verbatim so we never
// silently drop a field.
func normalizeUpdated(s string) string {
	if s == "" {
		return ""
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format("2006-01-02T15:04:05Z")
	}
	return s
}
