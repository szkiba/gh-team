package cmd

import (
	"github.com/spf13/cobra"
)

// newSecurityCmd returns the `security` command group, a sibling of `repo`.
// The group itself has no action; it dispatches to summary/alerts.
func newSecurityCmd(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security",
		Short: "Inspect open security alerts for repositories owned by a team",
		Long: `Subcommands that read open Dependabot and code-scanning alerts for the
repositories owned by a team under the active ownership strategy.

Security commands assume the caller has at least repository maintain
permission on each owned repository. That baseline maps cleanly to
--ownership=permission. With --ownership=codeowners the resolver may
return repositories the caller cannot read alerts for; those produce
per-repository warnings and a non-zero exit, while accessible
repositories still contribute output.

Supported alert families: dependabot, code-scanning. Secret scanning is
not part of the MVP. --kind=all is a fixed alias for the union of
dependabot and code-scanning.`,
		Example: `  gh team security summary octo/platform
  gh team security alerts octo/platform --kind=dependabot`,
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}
	cmd.AddCommand(newSecuritySummaryCmd(flags), newSecurityAlertsCmd(flags))
	return cmd
}
