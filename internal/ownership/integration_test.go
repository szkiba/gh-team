package ownership

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
)

// TestIntegration_PermissionFullFlow exercises the permission strategy
// through the public newResolver factory with a multi-team fixture set: a
// parent team plus two sub-teams, a mix of granted permissions including
// some that must be excluded, a duplicate repo reachable through both the
// parent and a sub-team, and an archived repo that must be filtered out by
// default.
func TestIntegration_PermissionFullFlow(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("orgs/octo/teams/platform/repos", staticJSON(`[
		{"name":"web","owner":{"login":"octo"},"permissions":{"admin":true}},
		{"name":"docs","owner":{"login":"octo"},"permissions":{"push":true}},
		{"name":"contrib","owner":{"login":"octo"},"permissions":{"triage":true}},
		{"name":"legacy","owner":{"login":"octo"},"archived":true,"permissions":{"admin":true}}
	]`))
	c.on("orgs/octo/teams/platform/teams", staticJSON(`[{"slug":"ingest"},{"slug":"api"}]`))
	c.on("orgs/octo/teams/ingest/repos", staticJSON(`[
		{"name":"ingestor","owner":{"login":"octo"},"permissions":{"maintain":true}},
		{"name":"web","owner":{"login":"octo"},"permissions":{"admin":true}}
	]`))
	c.on("orgs/octo/teams/ingest/teams", staticJSON(`[]`))
	c.on("orgs/octo/teams/api/repos", staticJSON(`[
		{"name":"api","owner":{"login":"octo"},"permissions":{"admin":true}}
	]`))
	c.on("orgs/octo/teams/api/teams", staticJSON(`[]`))

	res, err := newResolver(StrategyPermission, c, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	got, err := res.Resolve(context.Background(), "octo", "platform", Options{})
	if err != nil {
		t.Fatal(err)
	}
	// docs/contrib lack admin/maintain; legacy archived; web deduped; sorted.
	assertRepos(t, got, []string{"octo/api", "octo/ingestor", "octo/web"})
}

// TestIntegration_CodeownersFullFlow drives the codeowners strategy through
// the public factory with four candidate repos that cover every parser and
// path-resolution rule: .github precedence overriding root, fallback to
// docs/CODEOWNERS, a candidate with no file at any path, and a simple
// happy-path wildcard owner.
func TestIntegration_CodeownersFullFlow(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("search/code", staticJSON(`{"items":[
		{"repository":{"name":"api","owner":{"login":"octo"}}},
		{"repository":{"name":"web","owner":{"login":"octo"}}},
		{"repository":{"name":"docsite","owner":{"login":"octo"}}},
		{"repository":{"name":"stale","owner":{"login":"octo"}}}
	]}`))

	// api: .github overrides root — root would say platform, .github says other.
	c.on("repos/octo/api/contents/.github/CODEOWNERS", staticJSON(base64File("* @octo/other\n")))

	// web: simple wildcard owner in .github.
	c.on("repos/octo/web/contents/.github/CODEOWNERS", staticJSON(base64File("* @octo/platform\n")))
	c.on("repos/octo/web", staticJSON(repoMeta(false)))

	// docsite: docs/CODEOWNERS fallback after the two earlier paths 404.
	c.on("repos/octo/docsite/contents/.github/CODEOWNERS", notFound())
	c.on("repos/octo/docsite/contents/CODEOWNERS", notFound())
	c.on("repos/octo/docsite/contents/docs/CODEOWNERS", staticJSON(base64File("* @octo/platform\n")))
	c.on("repos/octo/docsite", staticJSON(repoMeta(false)))

	// stale: search returned it but the file is now gone everywhere.
	c.on("repos/octo/stale/contents/.github/CODEOWNERS", notFound())
	c.on("repos/octo/stale/contents/CODEOWNERS", notFound())
	c.on("repos/octo/stale/contents/docs/CODEOWNERS", notFound())

	var stderr bytes.Buffer
	res, err := newResolver(StrategyCodeowners, c, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	got, err := res.Resolve(context.Background(), "octo", "platform", Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertRepos(t, got, []string{"octo/docsite", "octo/web"})
	if !strings.Contains(stderr.String(), "code search index") {
		t.Errorf("expected search-index lag note on stderr, got %q", stderr.String())
	}
}
