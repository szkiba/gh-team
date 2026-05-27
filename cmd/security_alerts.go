package cmd

import (
	"fmt"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/ownership"
	"github.com/szkiba/gh-team/internal/security"
)

func newSecurityAlertsCmd(flags *globalFlags) *cobra.Command {
	var kindFlag string
	c := &cobra.Command{
		Use:   "alerts <org/team-slug>",
		Short: "List individual open security alerts across owned repositories",
		Long: `Resolve the repositories owned by the team and print one tab-separated
line per open alert across them.

Each line is "<family>\t<org>/<repo>\t<key>\t<severity>\t<url>". The
Dependabot key is "<ecosystem>:<package>@<manifest-path>"; the
code-scanning key is the rule id. Code-scanning severity prefers
security_severity_level and falls back to rule.severity.

Output is sorted by repository, family, key, then URL, with no header.
Use --kind to restrict to a single family.`,
		Example: `  gh team security alerts octo/platform
  gh team security alerts octo/platform --kind=code-scanning`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runSecurityAlerts(c, flags, args[0], kindFlag)
		},
	}
	c.Flags().StringVar(&kindFlag, "kind", kindFlagDefault,
		"alert family to query: dependabot|code-scanning|all")
	return c
}

func runSecurityAlerts(c *cobra.Command, flags *globalFlags, arg, kindFlag string) error {
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
	for _, row := range res.Alerts {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n",
			row.Family, row.Repo, row.Key, row.Severity, row.URL)
	}
	return emitWarningsAndExit(c, res)
}

// resolveForSecurity is the shared "parse arg → resolve repos" prefix used by
// both security subcommands. It hides the api.RESTClient and ownership wiring
// so the run functions stay focused on alert collection and rendering.
func resolveForSecurity(c *cobra.Command, flags *globalFlags, arg string) ([]ownership.Repo, error) {
	org, slug, err := parseOrgTeam(arg)
	if err != nil {
		return nil, err
	}
	resolver, opts, err := buildResolver(flags, c.ErrOrStderr())
	if err != nil {
		return nil, err
	}
	repos, err := resolver.Resolve(c.Context(), org, slug, opts)
	if err != nil {
		return nil, translateAPIError(err)
	}
	return repos, nil
}

// emitWarningsAndExit writes any per-repo access warnings to stderr and
// converts the collector's HardFailures count into a non-zero command exit.
// Returning a sentinel error here works with cobra's SilenceErrors so the
// warning lines are not duplicated as a final "Error: ..." print.
func emitWarningsAndExit(c *cobra.Command, res *security.Result) error {
	stderr := c.ErrOrStderr()
	for _, w := range res.Warnings {
		fmt.Fprintln(stderr, w)
	}
	if res.HardFailures > 0 {
		// Surface a terse, deterministic exit error. cobra prints it via
		// SilenceErrors=false in newRootCmd, but the substantive
		// per-repo information is already in the warnings above.
		return errSecurityIncomplete{count: res.HardFailures}
	}
	return nil
}

// errSecurityIncomplete is the exit-status carrier when the collector saw
// at least one access-level failure. The Error message is intentionally
// short because the per-repo warnings already explain what failed.
type errSecurityIncomplete struct {
	count int
}

func (e errSecurityIncomplete) Error() string {
	return fmt.Sprintf("%d repository/family pair(s) could not be read; see warnings above", e.count)
}

