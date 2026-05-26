package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/ownership"
)

func newRepoCloneCmd(flags *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "clone <org/team-slug>",
		Short: "Clone every repository owned by the team into the current directory",
		Long: `Resolve the team's owned repositories under the active ownership
strategy and run "gh repo clone <org>/<repo>" for each into a
subdirectory of the current working directory.

Pre-existing directories are skipped with a non-fatal warning to
stderr so a partial state is recoverable. Per-repository clone
failures are aggregated; the batch continues and the command exits
non-zero at the end if any clone failed.`,
		Example: `  # Clone every owned repository into the current directory
  gh team repo clone octo/platform

  # Use CODEOWNERS to determine ownership
  gh team repo clone octo/platform --ownership=codeowners`,
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
			return cloneAll(c, repos)
		},
	}
}

// cloneAll runs `gh repo clone` per repo, aggregating failures so a single
// broken repo does not abort the whole batch. Pre-existing directories are
// skipped with a non-fatal warning per spec.
func cloneAll(c *cobra.Command, repos []ownership.Repo) error {
	stderr := c.ErrOrStderr()
	stdout := c.OutOrStdout()

	var failed []string
	for _, r := range repos {
		// `gh repo clone owner/name` clones into a directory named "name"
		// by default. Match GitHub's own collision check.
		if _, err := os.Stat(r.Name); err == nil {
			fmt.Fprintf(stderr, "skip %s: directory %q already exists\n", r.FullName(), r.Name)
			continue
		}

		clone := exec.CommandContext(c.Context(), "gh", "repo", "clone", r.FullName())
		clone.Stdout = stdout
		clone.Stderr = stderr
		if err := clone.Run(); err != nil {
			fmt.Fprintf(stderr, "clone failed for %s: %v\n", r.FullName(), err)
			failed = append(failed, r.FullName())
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d repository clone(s) failed: %v", len(failed), failed)
	}
	return nil
}
