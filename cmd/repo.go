package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/ownership"
)

func newRepoCmd(flags *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "repo",
		Short: "Repository discovery subcommands",
		RunE: func(c *cobra.Command, _ []string) error {
			return c.Help()
		},
	}
	cmd.AddCommand(newRepoListCmd(flags), newRepoCloneCmd(flags))
	return cmd
}

// parseOrgTeam splits "<org>/<team-slug>" into its parts. Stricter checks
// (empty halves, multiple slashes) live in section 4; this function does
// just enough to drive the resolver and lets the validator there own the
// user-facing error wording.
func parseOrgTeam(arg string) (org, slug string, err error) {
	if strings.Count(arg, "/") != 1 {
		return "", "", fmt.Errorf("invalid argument %q: expected <org>/<team-slug>", arg)
	}
	parts := strings.SplitN(arg, "/", 2)
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid argument %q: expected <org>/<team-slug>", arg)
	}
	return parts[0], parts[1], nil
}

// buildResolver constructs the strategy-specific resolver for a command run.
// stderr is forwarded so the codeowners strategy can emit its search-index
// lag note where the user expects it (the command's own stderr stream).
func buildResolver(flags *globalFlags, stderr io.Writer) (ownership.Resolver, ownership.Options, error) {
	client, err := api.DefaultRESTClient()
	if err != nil {
		return nil, ownership.Options{}, fmt.Errorf("create GitHub REST client: %w", err)
	}
	res, err := ownership.NewResolver(ownership.Strategy(flags.ownership), client, stderr)
	if err != nil {
		return nil, ownership.Options{}, err
	}
	return res, ownership.Options{
		DirectOnly:      flags.directOnly,
		IncludeArchived: flags.includeArchived,
	}, nil
}
