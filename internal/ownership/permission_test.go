package ownership

import (
	"context"
	"strings"
	"testing"
)

func TestPermissionStrategy_AdminAndMaintainIncluded(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("orgs/octo/teams/platform/repos", staticJSON(`[
		{"name":"api","owner":{"login":"octo"},"permissions":{"admin":true}},
		{"name":"web","owner":{"login":"octo"},"permissions":{"maintain":true}},
		{"name":"contrib","owner":{"login":"octo"},"permissions":{"push":true}},
		{"name":"triage-only","owner":{"login":"octo"},"permissions":{"triage":true}}
	]`))
	c.on("orgs/octo/teams/platform/teams", staticJSON(`[]`))

	s := &permissionStrategy{client: c}
	got, err := s.Resolve(context.Background(), "octo", "platform", Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertRepos(t, got, []string{"octo/api", "octo/web"})
}

func TestPermissionStrategy_SubTeamRecursion(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("orgs/octo/teams/platform/repos", staticJSON(`[
		{"name":"api","owner":{"login":"octo"},"permissions":{"admin":true}}
	]`))
	c.on("orgs/octo/teams/platform/teams", staticJSON(`[{"slug":"ingest"}]`))
	c.on("orgs/octo/teams/ingest/repos", staticJSON(`[
		{"name":"ingestor","owner":{"login":"octo"},"permissions":{"maintain":true}}
	]`))
	c.on("orgs/octo/teams/ingest/teams", staticJSON(`[]`))

	s := &permissionStrategy{client: c}
	got, err := s.Resolve(context.Background(), "octo", "platform", Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertRepos(t, got, []string{"octo/api", "octo/ingestor"})
}

func TestPermissionStrategy_DirectOnlySkipsSubTeams(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("orgs/octo/teams/platform/repos", staticJSON(`[
		{"name":"api","owner":{"login":"octo"},"permissions":{"admin":true}}
	]`))
	// Sub-team handlers intentionally missing — must not be called.

	s := &permissionStrategy{client: c}
	got, err := s.Resolve(context.Background(), "octo", "platform", Options{DirectOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	assertRepos(t, got, []string{"octo/api"})
}

func TestPermissionStrategy_DedupesAcrossParentAndSubTeam(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("orgs/octo/teams/platform/repos", staticJSON(`[
		{"name":"api","owner":{"login":"octo"},"permissions":{"admin":true}}
	]`))
	c.on("orgs/octo/teams/platform/teams", staticJSON(`[{"slug":"sub"}]`))
	c.on("orgs/octo/teams/sub/repos", staticJSON(`[
		{"name":"api","owner":{"login":"octo"},"permissions":{"admin":true}}
	]`))
	c.on("orgs/octo/teams/sub/teams", staticJSON(`[]`))

	s := &permissionStrategy{client: c}
	got, err := s.Resolve(context.Background(), "octo", "platform", Options{})
	if err != nil {
		t.Fatal(err)
	}
	assertRepos(t, got, []string{"octo/api"})
}

// TestPermissionStrategy_SubTeamCycleSafe guards against a malformed sub-team
// graph claiming the parent as its own child — without the visited-set the
// BFS would loop forever.
func TestPermissionStrategy_SubTeamCycleSafe(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("orgs/octo/teams/platform/repos", staticJSON(`[]`))
	c.on("orgs/octo/teams/platform/teams", staticJSON(`[{"slug":"sub"}]`))
	c.on("orgs/octo/teams/sub/repos", staticJSON(`[]`))
	c.on("orgs/octo/teams/sub/teams", staticJSON(`[{"slug":"platform"}]`))

	s := &permissionStrategy{client: c}
	if _, err := s.Resolve(context.Background(), "octo", "platform", Options{}); err != nil {
		t.Fatal(err)
	}
}

func TestPermissionStrategy_ArchivedFilter(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", staticJSON(`{"slug":"platform"}`))
	c.on("orgs/octo/teams/platform/repos", staticJSON(`[
		{"name":"alive","owner":{"login":"octo"},"archived":false,"permissions":{"admin":true}},
		{"name":"legacy","owner":{"login":"octo"},"archived":true,"permissions":{"admin":true}}
	]`))
	c.on("orgs/octo/teams/platform/teams", staticJSON(`[]`))

	s := &permissionStrategy{client: c}

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

func TestPermissionStrategy_MissingTeamReturns404Error(t *testing.T) {
	c := newFakeClient()
	c.on("orgs/octo/teams/platform", notFound())

	s := &permissionStrategy{client: c}
	_, err := s.Resolve(context.Background(), "octo", "platform", Options{})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error %q does not name the missing-team cause", err)
	}
}

// assertRepos checks the resolver returned the expected full names in order.
// finalize() guarantees alphabetical sort, so the slices are compared as-is.
func assertRepos(t *testing.T, got []Repo, want []string) {
	t.Helper()
	gotNames := make([]string, len(got))
	for i, r := range got {
		gotNames[i] = r.FullName()
	}
	if len(gotNames) != len(want) {
		t.Fatalf("got %d repos %v, want %d %v", len(gotNames), gotNames, len(want), want)
	}
	for i := range want {
		if gotNames[i] != want[i] {
			t.Errorf("repo[%d] = %q, want %q (full got=%v want=%v)", i, gotNames[i], want[i], gotNames, want)
		}
	}
}
