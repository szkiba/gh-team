package cmd

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/security"
)

// kindFlagDefault is the canonical default for --kind. Pulled out as a
// constant so summary and alerts stay in sync if it ever changes.
const kindFlagDefault = string(security.KindAll)

func newSecuritySummaryCmd(flags *globalFlags) *cobra.Command {
	var kindFlag string
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
fixed alias for the union of dependabot and code-scanning.`,
		Example: `  gh team security summary octo/platform
  gh team security summary octo/platform --kind=dependabot`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runSecuritySummary(c, flags, args[0], kindFlag)
		},
	}
	c.Flags().StringVar(&kindFlag, "kind", kindFlagDefault,
		"alert family to query: dependabot|code-scanning|all")
	return c
}

func runSecuritySummary(c *cobra.Command, flags *globalFlags, arg, kindFlag string) error {
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

	out := c.OutOrStdout()
	for _, row := range res.Summary {
		fmt.Fprintf(out, "%s\t%s\t%d\n", row.Repo, row.Family, row.Count)
	}
	return emitWarningsAndExit(c, res)
}
