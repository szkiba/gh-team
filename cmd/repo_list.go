package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/ownership"
)

func newRepoListCmd(flags *globalFlags) *cobra.Command {
	out := &outputFlags{}
	cmd := &cobra.Command{
		Use:   "list <org/team-slug>",
		Short: "List repositories owned by the team, one per line",
		Long: `Print the full names ("<org>/<repo>") of every repository owned by the
team under the active ownership strategy, one per line on stdout,
sorted alphabetically and with no header or count so the output can be
piped directly into other commands.

Archived repositories are excluded unless --include-archived is set.
An empty result still exits 0.

Use --json for a JSON array of repository objects, or --template with a
Go text/template to render one custom line per repository. The two
flags are mutually exclusive. JSON and template fields: .owner, .name,
.full_name, .archived. Template mode requires every referenced field
to exist on the row context and rejects any rendering that produces
more than one line per repository.

Use --header in default TSV mode to prepend a single header line
("owner\tname\tfull_name\tarchived") and switch the data rows to the
same four-column TSV shape, so the output imports cleanly into Excel
or Google Sheets. The no-flag default mode is unchanged (one
"<org>/<repo>" per line). --header is rejected with --json or
--template.`,
		Example: `  # Default permission strategy
  gh team repo list octo/platform

  # CODEOWNERS strategy
  gh team repo list octo/platform --ownership=codeowners

  # Only repositories assigned directly to the top-level team
  gh team repo list octo/platform --direct-only

  # Include archived repositories
  gh team repo list octo/platform --include-archived

  # Pipe-friendly: feed the result into another command
  gh team repo list octo/platform | xargs -L1 gh repo view

  # JSON array for scripting
  gh team repo list octo/platform --json | jq '.[].full_name'

  # Custom one-line-per-repo rendering
  gh team repo list octo/platform --template '{{.full_name}} archived={{.archived}}'

  # Labeled TSV with header line for spreadsheet import
  gh team repo list octo/platform --header --include-archived`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			plan, err := out.resolve()
			if err != nil {
				return err
			}
			org, slug, err := parseOrgTeam(args[0])
			if err != nil {
				return err
			}
			resolver, opts, err := buildResolver(flags, c.ErrOrStderr())
			if err != nil {
				return err
			}
			repos, err := resolver.Resolve(c.Context(), org, slug, opts)
			if err != nil {
				return translateAPIError(err)
			}
			return plan.render(c.OutOrStdout(), repoRows(repos), renderConfig{
				header:      "owner\tname\tfull_name\tarchived",
				defFn:       renderRepoDefault,
				defHeaderFn: renderRepoWithHeaderColumns,
			})
		},
	}
	out.attach(cmd)
	return cmd
}

// repoRows projects the resolver's repo list into the public output
// contract. Field names are the JSON/template field names documented in
// the team-repo spec, so the same slice powers both modes.
func repoRows(repos []ownership.Repo) []map[string]any {
	rows := make([]map[string]any, 0, len(repos))
	for _, r := range repos {
		rows = append(rows, map[string]any{
			"owner":     r.Owner,
			"name":      r.Name,
			"full_name": r.FullName(),
			"archived":  r.Archived,
		})
	}
	return rows
}

// renderRepoDefault preserves the existing default behavior byte-for-byte:
// one `<org>/<repo>` per line, no header.
func renderRepoDefault(out io.Writer, row map[string]any) error {
	_, err := fmt.Fprintln(out, row["full_name"])
	return err
}

// renderRepoWithHeaderColumns widens each row to the four-column TSV named
// in the header — required by design Decision 0 so that the labeled
// output is structurally consistent. The archived cell is rendered as the
// lower-case string `true`/`false` so it lines up with the JSON boolean
// contract when the cells land in a spreadsheet column typed as boolean.
func renderRepoWithHeaderColumns(out io.Writer, row map[string]any) error {
	archived := "false"
	if v, _ := row["archived"].(bool); v {
		archived = "true"
	}
	_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\n",
		row["owner"], row["name"], row["full_name"], archived)
	return err
}
