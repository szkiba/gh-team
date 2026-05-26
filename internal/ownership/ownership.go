// Package ownership resolves a GitHub team to the set of repositories it owns.
//
// Two strategies are provided:
//   - permission: the team or any sub-team has Admin or Maintain on the repo.
//   - codeowners: the team owns the bare * line in the repo's effective CODEOWNERS.
//
// Both strategies share a single team-existence preflight so that stale or
// mistyped team slugs fail with the same error regardless of strategy.
package ownership

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Strategy names the algorithm used to determine ownership.
type Strategy string

const (
	StrategyPermission Strategy = "permission"
	StrategyCodeowners Strategy = "codeowners"
)

// Repo is the minimal repository view the resolver returns.
type Repo struct {
	Owner    string
	Name     string
	Archived bool
}

// FullName returns the canonical "<owner>/<name>" identifier.
func (r Repo) FullName() string { return r.Owner + "/" + r.Name }

// Options governs how a resolver behaves for a single invocation.
type Options struct {
	DirectOnly      bool
	IncludeArchived bool
}

// Resolver maps an org/team-slug to the repositories it owns.
type Resolver interface {
	Resolve(ctx context.Context, org, teamSlug string, opts Options) ([]Repo, error)
}

// restClient is the narrow subset of *api.RESTClient the strategies need.
// Defined as an interface so unit tests can substitute a fake without
// hitting the network.
type restClient interface {
	Get(path string, response interface{}) error
}

// NewResolver constructs the resolver for the named strategy. stderr is used
// only by the codeowners strategy to emit the one-line search-index-lag note.
func NewResolver(strategy Strategy, client *api.RESTClient, stderr io.Writer) (Resolver, error) {
	return newResolver(strategy, client, stderr)
}

// newResolver is the internal constructor that takes the narrow restClient
// interface, so tests can substitute a fake without an HTTP server. The
// public NewResolver delegates here, so both production and test paths
// build the same strategy values.
func newResolver(strategy Strategy, client restClient, stderr io.Writer) (Resolver, error) {
	switch strategy {
	case StrategyPermission:
		return &permissionStrategy{client: client}, nil
	case StrategyCodeowners:
		return &codeownersStrategy{client: client, stderr: stderr}, nil
	default:
		return nil, fmt.Errorf("unknown ownership strategy %q", strategy)
	}
}

// finalize applies the archived filter, deduplicates by full name, and sorts
// alphabetically. Both strategies feed their raw output through this so the
// post-processing rules are guaranteed identical.
func finalize(repos []Repo, includeArchived bool) []Repo {
	seen := make(map[string]Repo, len(repos))
	for _, r := range repos {
		if !includeArchived && r.Archived {
			continue
		}
		seen[r.FullName()] = r
	}
	out := make([]Repo, 0, len(seen))
	for _, r := range seen {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].FullName() < out[j].FullName()
	})
	return out
}
