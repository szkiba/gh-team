package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newRepoListCmd(flags *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "list <org/team-slug>",
		Short: "List repositories owned by the team, one per line",
		Args:  cobra.ExactArgs(1),
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
				return err
			}
			out := c.OutOrStdout()
			for _, r := range repos {
				fmt.Fprintln(out, r.FullName())
			}
			return nil
		},
	}
}
