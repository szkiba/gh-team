package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRepoListCmd(flags *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list <org/team-slug>",
		Short: "List repositories owned by the team, one per line",
		Long: `Print the full names ("<org>/<repo>") of every repository owned by the
team under the active ownership strategy, one per line on stdout,
sorted alphabetically and with no header or count so the output can be
piped directly into other commands.

Archived repositories are excluded unless --include-archived is set.
An empty result still exits 0.`,
		Example: `  # Default permission strategy
  gh team repo list octo/platform

  # CODEOWNERS strategy
  gh team repo list octo/platform --ownership=codeowners

  # Only repositories assigned directly to the top-level team
  gh team repo list octo/platform --direct-only

  # Include archived repositories
  gh team repo list octo/platform --include-archived

  # Pipe-friendly: feed the result into another command
  gh team repo list octo/platform | xargs -L1 gh repo view`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
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
			out := c.OutOrStdout()
			for _, r := range repos {
				fmt.Fprintln(out, r.FullName())
			}
			return nil
		},
	}
}
