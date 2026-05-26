package ownership

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// codeownersStrategy resolves ownership through CODEOWNERS files. It is a
// two-step pipeline: GitHub code search supplies candidate repos, then each
// candidate's effective CODEOWNERS file is fetched and parsed exactly. The
// search step is best-effort; the per-candidate decision is exact.
type codeownersStrategy struct {
	client restClient
	stderr io.Writer
}

// codeOwnersPaths is the resolution order GitHub itself uses. The first file
// that exists is the effective one; later paths are ignored.
var codeOwnersPaths = []string{".github/CODEOWNERS", "CODEOWNERS", "docs/CODEOWNERS"}

// errNoCodeowners signals that none of the candidate paths existed on the
// default branch — the candidate is rejected, not propagated.
var errNoCodeowners = errors.New("no CODEOWNERS file at any known path")

func (s *codeownersStrategy) Resolve(_ context.Context, org, teamSlug string, opts Options) ([]Repo, error) {
	if _, err := preflight(s.client, org, teamSlug); err != nil {
		return nil, err
	}

	// Per spec, the search-index-lag note SHALL be emitted on every codeowners
	// invocation. stdout and exit status are unaffected.
	fmt.Fprintln(s.stderr, "note: --ownership=codeowners results come from GitHub's code search index; recently added or renamed CODEOWNERS files may be missing until they are re-indexed.")

	candidates, err := s.searchCandidates(org, teamSlug)
	if err != nil {
		return nil, err
	}

	var owned []Repo
	for _, c := range candidates {
		content, err := s.fetchEffectiveCodeowners(c.Owner, c.Name)
		if err != nil {
			if errors.Is(err, errNoCodeowners) {
				continue
			}
			return nil, err
		}
		if !teamOwnsWildcard(content, org, teamSlug) {
			continue
		}
		// Archived state isn't populated by the code-search response's
		// minimal-repo object, and GitHub's code search rejects an
		// "archived:false" qualifier outright (returning zero hits), so it
		// must be resolved with a per-candidate metadata fetch. We defer it
		// until after the parse decision so non-owning candidates do not
		// pay the extra REST call.
		archived, err := s.fetchRepoArchived(c.Owner, c.Name)
		if err != nil {
			return nil, err
		}
		c.Archived = archived
		owned = append(owned, c)
	}
	return finalize(owned, opts.IncludeArchived), nil
}

type searchCodeResponse struct {
	TotalCount int `json:"total_count"`
	Items      []struct {
		Repository struct {
			Name  string `json:"name"`
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
	} `json:"items"`
}

// searchCandidates issues the broad team-mention query against CODEOWNERS
// files in the org. The broad form (just the team mention, no `*` constraint)
// is required so wildcard lines with multiple owners or unusual whitespace are
// not missed at the candidate stage. Per-repo exactness is enforced later.
//
// Two query-syntax details matter:
//   - filename:CODEOWNERS, not path:CODEOWNERS. GitHub's code search treats
//     `path:` as an exact path match against the literal "CODEOWNERS" string
//     and returns zero hits for the files that actually live at
//     .github/CODEOWNERS or docs/CODEOWNERS.
//   - The archived filter cannot be pushed into the query. The
//     "archived:false" qualifier is only honored by repo-search; in
//     code-search it causes GitHub to return zero hits. Archive state is
//     resolved per owned candidate above.
func (s *codeownersStrategy) searchCandidates(org, teamSlug string) ([]Repo, error) {
	q := fmt.Sprintf(`org:%s filename:CODEOWNERS "@%s/%s"`, org, org, teamSlug)
	encoded := url.QueryEscape(q)

	seen := make(map[string]struct{})
	var out []Repo
	for page := 1; ; page++ {
		path := fmt.Sprintf("search/code?per_page=%d&page=%d&q=%s", pageSize, page, encoded)
		var resp searchCodeResponse
		if err := s.client.Get(path, &resp); err != nil {
			return nil, err
		}
		for _, item := range resp.Items {
			r := Repo{Owner: item.Repository.Owner.Login, Name: item.Repository.Name}
			if _, dup := seen[r.FullName()]; dup {
				continue
			}
			seen[r.FullName()] = struct{}{}
			out = append(out, r)
		}
		if len(resp.Items) < pageSize {
			return out, nil
		}
	}
}

type contentsResponse struct {
	Content  string `json:"content"`
	Encoding string `json:"encoding"`
}

// fetchEffectiveCodeowners walks .github/CODEOWNERS, CODEOWNERS, then
// docs/CODEOWNERS on the default branch and returns the first existing file.
// 404 on a path means "try the next"; any other error is fatal.
func (s *codeownersStrategy) fetchEffectiveCodeowners(owner, repo string) (string, error) {
	for _, p := range codeOwnersPaths {
		var c contentsResponse
		err := s.client.Get(fmt.Sprintf("repos/%s/%s/contents/%s", owner, repo, p), &c)
		if err != nil {
			var herr *api.HTTPError
			if errors.As(err, &herr) && herr.StatusCode == 404 {
				continue
			}
			return "", err
		}
		if c.Encoding != "base64" {
			return "", fmt.Errorf("unexpected content encoding %q for %s/%s/%s", c.Encoding, owner, repo, p)
		}
		// GitHub embeds line breaks every 60 chars in base64 content; strip them.
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(c.Content, "\n", ""))
		if err != nil {
			return "", fmt.Errorf("decode CODEOWNERS for %s/%s: %w", owner, repo, err)
		}
		return string(decoded), nil
	}
	return "", errNoCodeowners
}

// fetchRepoArchived returns the `archived` flag for a single repository. It
// is called for every owned candidate so the archived filter can run; the
// minimal-repo object that code search returns does not include this field.
func (s *codeownersStrategy) fetchRepoArchived(owner, name string) (bool, error) {
	var resp struct {
		Archived bool `json:"archived"`
	}
	if err := s.client.Get(fmt.Sprintf("repos/%s/%s", owner, name), &resp); err != nil {
		return false, err
	}
	return resp.Archived, nil
}
