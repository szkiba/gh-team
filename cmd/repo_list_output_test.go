package cmd

import (
	"bytes"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/szkiba/gh-team/internal/ownership"
)

func renderReposWithPlan(t *testing.T, of *outputFlags, repos []ownership.Repo) string {
	t.Helper()
	plan, err := of.resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	var buf bytes.Buffer
	if err := plan.render(&buf, repoRows(repos), renderConfig{header: "owner\tname\tfull_name\tarchived", defFn: renderRepoDefault, defHeaderFn: renderRepoWithHeaderColumns}); err != nil {
		t.Fatalf("render: %v", err)
	}
	return buf.String()
}

func TestRepoList_DefaultByteCompat(t *testing.T) {
	repos := []ownership.Repo{
		{Owner: "octo", Name: "api"},
		{Owner: "octo", Name: "ingestor"},
		{Owner: "octo", Name: "web"},
	}
	got := renderReposWithPlan(t, &outputFlags{}, repos)
	want := "octo/api\nocto/ingestor\nocto/web\n"
	if got != want {
		t.Errorf("default repo-list output drifted:\n got %q\nwant %q", got, want)
	}
}

func TestRepoList_JSONShape(t *testing.T) {
	repos := []ownership.Repo{{Owner: "octo", Name: "api", Archived: false}}
	got := renderReposWithPlan(t, &outputFlags{json: true}, repos)
	var arr []map[string]any
	if err := json.Unmarshal([]byte(got), &arr); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if arr[0]["full_name"] != "octo/api" || arr[0]["archived"] != false {
		t.Errorf("JSON shape wrong: %v", arr)
	}
}

// TestRepoList_TemplatePreservesDefaultOrdering is the regression for the
// spec scenario `Template output preserves default ordering` on
// `gh team repo list`.
func TestRepoList_TemplatePreservesDefaultOrdering(t *testing.T) {
	repos := []ownership.Repo{
		{Owner: "octo", Name: "api"},
		{Owner: "octo", Name: "ingestor"},
		{Owner: "octo", Name: "web"},
	}
	got := renderReposWithPlan(t, &outputFlags{template: "{{.full_name}}"}, repos)
	want := "octo/api\nocto/ingestor\nocto/web\n"
	if got != want {
		t.Errorf("template order mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestRepoRows_FieldShape(t *testing.T) {
	rows := repoRows([]ownership.Repo{{Owner: "octo", Name: "api", Archived: true}})
	want := []map[string]any{
		{"owner": "octo", "name": "api", "full_name": "octo/api", "archived": true},
	}
	if !reflect.DeepEqual(rows, want) {
		t.Errorf("rows = %v, want %v", rows, want)
	}
}

// TestRepoClone_NoOutputFlags is a positive guard for task 2.4: the clone
// subcommand intentionally does not get --json or --template. If anyone
// adds those flags by accident this test will catch it.
func TestRepoClone_NoOutputFlags(t *testing.T) {
	cmd := newRepoCloneCmd(&globalFlags{ownership: ownershipPermission})
	for _, name := range []string{"json", "template"} {
		if cmd.Flags().Lookup(name) != nil {
			t.Errorf("repo clone must not expose --%s", name)
		}
	}
}
