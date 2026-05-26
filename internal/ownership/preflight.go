package ownership

import (
	"errors"
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
)

// teamRef is the slice of GET /orgs/{org}/teams/{slug} that the resolvers care
// about. Extending it later (e.g. with parent/privacy fields) is safe.
type teamRef struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

// preflight verifies that org/slug names a real team in the org. It is
// intentionally identical for both strategies so that mistyped or stale
// slugs fail with the same error regardless of --ownership.
func preflight(c restClient, org, slug string) (*teamRef, error) {
	var t teamRef
	if err := c.Get(fmt.Sprintf("orgs/%s/teams/%s", org, slug), &t); err != nil {
		var herr *api.HTTPError
		if errors.As(err, &herr) && herr.StatusCode == 404 {
			return nil, fmt.Errorf("team %q does not exist in organization %q (or is not visible to the authenticated session)", slug, org)
		}
		return nil, err
	}
	return &t, nil
}
