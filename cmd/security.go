package cmd

import (
	"github.com/spf13/cobra"
)

// newSecurityCmd returns the `security` command group, a sibling of `repo`.
// The group itself has no action; it dispatches to summary/alerts.
func newSecurityCmd(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "security",
		Short: "Inspect security alerts and security pull requests for repositories owned by a team",
		Long: `Subcommands that read security data for the repositories owned by a
team under the active ownership strategy. The alerts subcommands cover
open Dependabot and code-scanning alerts; the prs subcommand lists open
pull requests whose title or labels match a security signal.

Security commands assume the caller has at least repository maintain
permission on each owned repository. That baseline maps cleanly to
--ownership=permission. With --ownership=codeowners the resolver may
return repositories the caller cannot read security data for (alerts
for summary/alerts, pull requests for prs); those produce
per-repository warnings and a non-zero exit, while accessible
repositories still contribute output.

Supported alert families: dependabot, code-scanning. Secret scanning is
not part of the MVP. --kind=all is a fixed alias for the union of
dependabot and code-scanning.

Use the prs subcommand to list open pull requests across the team's
owned repositories that match a security signal (title regex or
label). Listing pull requests on private repositories requires
repository-read access (classic OAuth scope "repo", or fine-grained
"Pull requests: read") on top of "read:org".`,
		Example: `  gh team security summary octo/platform
  gh team security alerts octo/platform --kind=dependabot
  gh team security prs octo/platform --header`,
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}
	cmd.AddCommand(newSecuritySummaryCmd(flags), newSecurityAlertsCmd(flags), newSecurityPrsCmd(flags))
	return cmd
}
