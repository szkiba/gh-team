package ownership

import (
	"context"
	"fmt"
)

// permissionStrategy resolves ownership through GitHub's team-to-repo
// permission bindings. A repo is owned when the team — or any sub-team
// (unless --direct-only) — has Admin or Maintain permission on it.
type permissionStrategy struct {
	client restClient
}

type ghRepoResponse struct {
	Name        string `json:"name"`
	Owner       struct {
		Login string `json:"login"`
	} `json:"owner"`
	Archived    bool `json:"archived"`
	Permissions struct {
		Admin    bool `json:"admin"`
		Maintain bool `json:"maintain"`
	} `json:"permissions"`
}

type ghTeamResponse struct {
	Slug string `json:"slug"`
}

const pageSize = 100

func (s *permissionStrategy) Resolve(_ context.Context, org, teamSlug string, opts Options) ([]Repo, error) {
	if _, err := preflight(s.client, org, teamSlug); err != nil {
		return nil, err
	}

	visited := map[string]struct{}{teamSlug: {}}
	queue := []string{teamSlug}
	var collected []Repo

	for len(queue) > 0 {
		slug := queue[0]
		queue = queue[1:]

		repos, err := s.listTeamRepos(org, slug)
		if err != nil {
			return nil, err
		}
		collected = append(collected, repos...)

		if opts.DirectOnly {
			continue
		}
		subs, err := s.listSubTeams(org, slug)
		if err != nil {
			return nil, err
		}
		for _, sub := range subs {
			if _, seen := visited[sub]; seen {
				continue
			}
			visited[sub] = struct{}{}
			queue = append(queue, sub)
		}
	}
	return finalize(collected, opts.IncludeArchived), nil
}

// listTeamRepos fetches all repos the team is bound to and keeps only those
// where the binding grants Admin or Maintain. Push/Pull/Triage are excluded.
func (s *permissionStrategy) listTeamRepos(org, slug string) ([]Repo, error) {
	var out []Repo
	for page := 1; ; page++ {
		var batch []ghRepoResponse
		path := fmt.Sprintf("orgs/%s/teams/%s/repos?per_page=%d&page=%d", org, slug, pageSize, page)
		if err := s.client.Get(path, &batch); err != nil {
			return nil, err
		}
		for _, r := range batch {
			if !(r.Permissions.Admin || r.Permissions.Maintain) {
				continue
			}
			out = append(out, Repo{Owner: r.Owner.Login, Name: r.Name, Archived: r.Archived})
		}
		if len(batch) < pageSize {
			return out, nil
		}
	}
}

// listSubTeams returns the slugs of every immediate child team. Recursion is
// handled by the caller through a BFS queue + visited set so the call graph
// stays flat and cycle-safe.
func (s *permissionStrategy) listSubTeams(org, slug string) ([]string, error) {
	var slugs []string
	for page := 1; ; page++ {
		var batch []ghTeamResponse
		path := fmt.Sprintf("orgs/%s/teams/%s/teams?per_page=%d&page=%d", org, slug, pageSize, page)
		if err := s.client.Get(path, &batch); err != nil {
			return nil, err
		}
		for _, t := range batch {
			slugs = append(slugs, t.Slug)
		}
		if len(batch) < pageSize {
			return slugs, nil
		}
	}
}
