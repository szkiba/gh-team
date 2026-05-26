package ownership

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
)

// repoMeta is the JSON body returned by GET /repos/{owner}/{name} that the
// codeowners strategy looks at — only the archived flag is consulted.
func repoMeta(archived bool) string {
	if archived {
		return `{"archived":true}`
	}
	return `{"archived":false}`
}

func TestCodeownersStrategy_WildcardOwnedAndLagNoteEmitted(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("search/code", staticJSON(`{"items":[{"repository":{"name":"api","owner":{"login":"octo"}}}]}`))
	c.on("repos/octo/api/contents/.github/CODEOWNERS", staticJSON(base64File("* @octo/platform\n")))
	c.on("repos/octo/api", staticJSON(repoMeta(false)))

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
			// Repo metadata is only fetched for owned candidates, so this
			// route is only hit in the wantRepo cases. Register it
			// unconditionally for simplicity.
			c.on("repos/octo/api", staticJSON(repoMeta(false)))

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

// TestCodeownersStrategy_NoArchivedQualifierInQuery locks in the fix for the
// "codeowners returned 0" bug. GitHub's code search returns zero hits when
// "archived:false" is in the query (the qualifier is only honored by
// repo-search), so the strategy MUST NOT push the archived filter into the
// query under any options.
func TestCodeownersStrategy_NoArchivedQualifierInQuery(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))

	var capturedQuery string
	c.on("search/code", func(q string) (string, *api.HTTPError) {
		capturedQuery = q
		return `{"items":[]}`, nil
	})

	var stderr bytes.Buffer
	s := &codeownersStrategy{client: c, stderr: &stderr}

	for _, opts := range []Options{{IncludeArchived: false}, {IncludeArchived: true}} {
		capturedQuery = ""
		if _, err := s.Resolve(context.Background(), "octo", "platform", opts); err != nil {
			t.Fatalf("Resolve with %+v: %v", opts, err)
		}
		if strings.Contains(capturedQuery, "archived") {
			t.Errorf("search query must not include an archived qualifier (opts=%+v); got %q",
				opts, capturedQuery)
		}
		if !strings.Contains(capturedQuery, "filename%3ACODEOWNERS") {
			t.Errorf("search query must use filename:CODEOWNERS, not path:CODEOWNERS (opts=%+v); got %q",
				opts, capturedQuery)
		}
	}
}

// TestCodeownersStrategy_ArchivedFilterViaMetadata verifies that archived
// repos are filtered through the per-candidate repo-metadata fetch (not the
// search query), and that --include-archived bypasses the filter while still
// running the metadata fetch.
func TestCodeownersStrategy_ArchivedFilterViaMetadata(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("search/code", staticJSON(`{"items":[
		{"repository":{"name":"alive","owner":{"login":"octo"}}},
		{"repository":{"name":"legacy","owner":{"login":"octo"}}}
	]}`))
	c.on("repos/octo/alive/contents/.github/CODEOWNERS", staticJSON(base64File("* @octo/platform\n")))
	c.on("repos/octo/legacy/contents/.github/CODEOWNERS", staticJSON(base64File("* @octo/platform\n")))
	c.on("repos/octo/alive", staticJSON(repoMeta(false)))
	c.on("repos/octo/legacy", staticJSON(repoMeta(true)))

	s := &codeownersStrategy{client: c, stderr: new(bytes.Buffer)}

	got, err := s.Resolve(context.Background(), "octo", "platform", Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertRepos(t, got, []string{"octo/alive"})

	got, err = s.Resolve(context.Background(), "octo", "platform", Options{IncludeArchived: true})
	if err != nil {
		t.Fatal(err)
	}
	assertRepos(t, got, []string{"octo/alive", "octo/legacy"})
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
