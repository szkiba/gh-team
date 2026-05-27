package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

const (
	ownershipPermission = "permission"
	ownershipCodeowners = "codeowners"
)

type globalFlags struct {
	ownership       string
	directOnly      bool
	includeArchived bool
}

func newRootCmd() *cobra.Command {
	flags := &globalFlags{}

	root := &cobra.Command{
		Use:   "team",
		Short: "Discover repositories and security alerts owned by a GitHub team",
		Long: `gh team lists or clones the repositories owned by a GitHub team and
inspects open Dependabot and code-scanning alerts across them.

Ownership is resolved through a configurable Team Ownership Model:
  - permission (default): the team or any sub-team has Admin or Maintain
    permission on the repository.
  - codeowners: the team appears on the last bare "*" rule in the
    repository's effective CODEOWNERS file on the default branch.

Security subcommands assume the caller has at least repository maintain
permission on each owned repository. That baseline maps cleanly to
--ownership=permission; with --ownership=codeowners the resolver may
return repositories the caller cannot read alerts for, which yield
per-repository warnings and a non-zero exit while accessible
repositories still contribute output.

The team argument is always "<org>/<team-slug>". Authentication and
rate limits are inherited from the host gh CLI session; sign in with
"gh auth login" first.`,
		Example: `  # List repositories the platform team owns (permission strategy)
  gh team repo list octo/platform

  # List repositories using CODEOWNERS instead
  gh team repo list octo/platform --ownership=codeowners

  # Clone every owned repository into the current directory
  gh team repo clone octo/platform

  # Summarize open Dependabot and code-scanning alerts
  gh team security summary octo/platform

  # List individual open code-scanning alerts
  gh team security alerts octo/platform --kind=code-scanning`,
		SilenceUsage:  true,
		SilenceErrors: false,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return validateGlobalFlags(flags)
		},
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}

	root.PersistentFlags().StringVar(&flags.ownership, "ownership", ownershipPermission,
		"ownership strategy: permission|codeowners")
	root.PersistentFlags().BoolVar(&flags.directOnly, "direct-only", false,
		"evaluate only direct team assignments (permission strategy only)")
	root.PersistentFlags().BoolVar(&flags.includeArchived, "include-archived", false,
		"include archived repositories")

	root.AddCommand(newRepoCmd(flags), newSecurityCmd(flags))

	return root
}

func validateGlobalFlags(f *globalFlags) error {
	switch f.ownership {
	case ownershipPermission, ownershipCodeowners:
	default:
		return fmt.Errorf("invalid --ownership %q: expected %q or %q",
			f.ownership, ownershipPermission, ownershipCodeowners)
	}
	if f.directOnly && f.ownership == ownershipCodeowners {
		return fmt.Errorf("--direct-only is only valid with --ownership=%s", ownershipPermission)
	}
	return nil
}

func Execute() error {
	return newRootCmd().Execute()
}
