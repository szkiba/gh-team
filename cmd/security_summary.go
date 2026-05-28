package cmd

import (
	"fmt"
	"io"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/security"
)

// kindFlagDefault is the canonical default for --kind. Pulled out as a
// constant so summary and alerts stay in sync if it ever changes.
const kindFlagDefault = string(security.KindAll)

func newSecuritySummaryCmd(flags *globalFlags) *cobra.Command {
	var kindFlag string
	out := &outputFlags{}
	c := &cobra.Command{
		Use:   "summary <org/team-slug>",
		Short: "Print open alert counts per owned repository and alert family",
		Long: `Resolve the repositories owned by the team and print a tab-separated
summary line for each (repository, alert family) pair that has at least
one open alert.

Each line is "<org>/<repo>\t<family>\t<count>". Output is sorted by
repository, then by family, with no header. Repositories where a
family has zero open alerts contribute no output line.

Use --kind to restrict to a single family. The default --kind=all is a
fixed alias for the union of dependabot and code-scanning.

Use --json for a JSON array of summary objects, or --template with a
Go text/template to render one custom line per item. The two flags
are mutually exclusive. JSON and template fields: .repo, .family,
.count. Items appear in the same repo-then-family order in every mode.

Use --header in default TSV mode to prepend a single header line
("repo\tfamily\tcount") for spreadsheet import. --header is rejected
with --json or --template.`,
		Example: `  gh team security summary octo/platform
  gh team security summary octo/platform --kind=dependabot
  gh team security summary octo/platform --json
  gh team security summary octo/platform --template '{{.repo}} {{.family}}={{.count}}'
  gh team security summary octo/platform --header`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runSecuritySummary(c, flags, out, args[0], kindFlag)
		},
	}
	c.Flags().StringVar(&kindFlag, "kind", kindFlagDefault,
		"alert family to query: dependabot|code-scanning|all")
	out.attach(c)
	return c
}

func runSecuritySummary(c *cobra.Command, flags *globalFlags, out *outputFlags, arg, kindFlag string) error {
	plan, err := out.resolve()
	if err != nil {
		return err
	}
	kind, err := security.ParseKind(kindFlag)
	if err != nil {
		return err
	}
	repos, err := resolveForSecurity(c, flags, arg)
	if err != nil {
		return err
	}
	client, err := api.DefaultRESTClient()
	if err != nil {
		return fmt.Errorf("create GitHub REST client: %w", err)
	}
	col := &security.Collector{Client: client}
	res, err := col.Collect(c.Context(), repos, kind.Families())
	if err != nil {
		return translateAPIError(err)
	}

	// Emit warnings before rendering so a render-time failure (template
	// parse, embedded-newline rejection) cannot swallow per-repo
	// diagnostics the team-security spec requires on stderr.
	emitSecurityWarnings(c, res)
	if err := plan.render(c.OutOrStdout(), summaryRows(res.Summary), renderConfig{
		header: "repo\tfamily\tcount",
		defFn:  renderSummaryDefault,
	}); err != nil {
		return err
	}
	return securityExitStatus(res)
}

// summaryRows projects collector summary rows into the public output
// contract — field names match the team-security spec for summary mode.
func summaryRows(rows []security.SummaryRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"repo":   r.Repo,
			"family": string(r.Family),
			"count":  r.Count,
		})
	}
	return out
}

func renderSummaryDefault(out io.Writer, row map[string]any) error {
	_, err := fmt.Fprintf(out, "%s\t%s\t%d\n", row["repo"], row["family"], row["count"])
	return err
}
