package ownership

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

func TestCodeownersStrategy_WildcardOwnedAndLagNoteEmitted(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("search/code", staticJSON(`{"items":[{"repository":{"name":"api","owner":{"login":"octo"}}}]}`))
	c.on("repos/octo/api/contents/.github/CODEOWNERS", staticJSON(base64File("* @octo/platform\n")))

	var stderr bytes.Buffer
	s := &codeownersStrategy{client: c, stderr: &stderr}
	got, err := s.Resolve(context.Background(), "octo", "platform", Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertRepos(t, got, []string{"octo/api"})
	if !strings.Contains(stderr.String(), "code search index") {
		t.Errorf("expected search-index lag note on stderr, got %q", stderr.String())
	}
}

func TestCodeownersStrategy_DotGithubTakesPrecedenceOverRoot(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("search/code", staticJSON(`{"items":[{"repository":{"name":"api","owner":{"login":"octo"}}}]}`))
	// .github wins — root would establish ownership but must be ignored.
	c.on("repos/octo/api/contents/.github/CODEOWNERS", staticJSON(base64File("* @octo/other\n")))
	c.on("repos/octo/api/contents/CODEOWNERS", staticJSON(base64File("* @octo/platform\n")))

	var stderr bytes.Buffer
	s := &codeownersStrategy{client: c, stderr: &stderr}
	got, err := s.Resolve(context.Background(), "octo", "platform", Options{})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want empty (.github/CODEOWNERS should have been the effective file)", got)
	}
}

func TestCodeownersStrategy_FallbackToRootAndDocs(t *testing.T) {
	cases := []struct {
		name     string
		paths    map[string]string // path -> file content (omit to 404)
		wantRepo bool
	}{
		{
			name: "root used when .github missing",
			paths: map[string]string{
				"repos/octo/api/contents/CODEOWNERS": "* @octo/platform\n",
			},
			wantRepo: true,
		},
		{
			name: "docs used when .github and root missing",
			paths: map[string]string{
				"repos/octo/api/contents/docs/CODEOWNERS": "* @octo/platform\n",
			},
			wantRepo: true,
		},
		{
			name:     "no file at any path drops candidate",
			paths:    map[string]string{},
			wantRepo: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newFakeClient()
			c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
			c.on("search/code", staticJSON(`{"items":[{"repository":{"name":"api","owner":{"login":"octo"}}}]}`))
			for _, p := range []string{
				"repos/octo/api/contents/.github/CODEOWNERS",
				"repos/octo/api/contents/CODEOWNERS",
				"repos/octo/api/contents/docs/CODEOWNERS",
			} {
				if body, ok := tc.paths[p]; ok {
					c.on(p, staticJSON(base64File(body)))
				} else {
					c.on(p, notFound())
				}
			}

			var stderr bytes.Buffer
			s := &codeownersStrategy{client: c, stderr: &stderr}
			got, err := s.Resolve(context.Background(), "octo", "platform", Options{})
			if err != nil {
				t.Fatal(err)
			}
			if tc.wantRepo {
				assertRepos(t, got, []string{"octo/api"})
			} else if len(got) != 0 {
				t.Errorf("got %v, want empty", got)
			}
		})
	}
}

// TestCodeownersStrategy_ArchivedQualifierConditional locks in the cost
// optimization where the archived filter is pushed down into the search
// query when --include-archived is off, saving one repo-metadata fetch per
// candidate. Reverting that optimization should fail this test.
func TestCodeownersStrategy_ArchivedQualifierConditional(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))

	var capturedQuery string
	c.on("search/code", func(q string) (string, *api.HTTPError) {
		capturedQuery = q
		return `{"items":[]}`, nil
	})

	var stderr bytes.Buffer
	s := &codeownersStrategy{client: c, stderr: &stderr}

	if _, err := s.Resolve(context.Background(), "octo", "platform", Options{IncludeArchived: false}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(capturedQuery, "archived%3Afalse") {
		t.Errorf("expected archived:false qualifier in search query, got %q", capturedQuery)
	}

	capturedQuery = ""
	if _, err := s.Resolve(context.Background(), "octo", "platform", Options{IncludeArchived: true}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(capturedQuery, "archived%3Afalse") {
		t.Errorf("did not expect archived:false qualifier when IncludeArchived is set, got %q", capturedQuery)
	}
}

func TestCodeownersStrategy_MissingTeamReturns404Error(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", notFound())

	var stderr bytes.Buffer
	s := &codeownersStrategy{client: c, stderr: &stderr}
	_, err := s.Resolve(context.Background(), "octo", "platform", Options{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q does not name the missing-team cause", err)
	}
	// No code search query should have run if preflight failed.
	if stderr.Len() != 0 {
		t.Errorf("preflight failure should suppress the lag note; got %q", stderr.String())
	}
}
