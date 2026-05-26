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
		Use:           "team",
		Short:         "Discover repositories owned by a GitHub team",
		Long:          "gh team lists or clones repositories owned by a GitHub team, using a configurable ownership strategy.",
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
