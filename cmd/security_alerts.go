package cmd

import (
	"fmt"
	"io"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/ownership"
	"github.com/szkiba/gh-team/internal/security"
)

func newSecurityAlertsCmd(flags *globalFlags) *cobra.Command {
	var kindFlag string
	out := &outputFlags{}
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
Use --kind to restrict to a single family.

Use --json for a JSON array of alert objects, or --template with a Go
text/template to render one custom line per alert. The two flags are
mutually exclusive. JSON and template fields: .family, .repo, .key,
.severity, .url. Items appear in the same sorted order in every mode.`,
		Example: `  gh team security alerts octo/platform
  gh team security alerts octo/platform --kind=code-scanning
  gh team security alerts octo/platform --json
  gh team security alerts octo/platform --template '{{.severity}} {{.repo}} {{.url}}'`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runSecurityAlerts(c, flags, out, args[0], kindFlag)
		},
	}
	c.Flags().StringVar(&kindFlag, "kind", kindFlagDefault,
		"alert family to query: dependabot|code-scanning|all")
	out.attach(c)
	return c
}

func runSecurityAlerts(c *cobra.Command, flags *globalFlags, out *outputFlags, arg, kindFlag string) error {
	plan, err := out.resolve()
	if err != nil {
		return err
	}
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

	// Emit warnings before rendering so a render-time failure (template
	// parse, embedded-newline rejection) cannot swallow per-repo
	// diagnostics the team-security spec requires on stderr.
	emitSecurityWarnings(c, res)
	if err := plan.render(c.OutOrStdout(), alertRows(res.Alerts), renderAlertDefault); err != nil {
		return err
	}
	return securityExitStatus(res)
}

// alertRows projects collector alert rows into the public output contract.
// Field names match the team-security spec for alerts mode.
func alertRows(rows []security.AlertRow) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"family":   string(r.Family),
			"repo":     r.Repo,
			"key":      r.Key,
			"severity": r.Severity,
			"url":      r.URL,
		})
	}
	return out
}

func renderAlertDefault(out io.Writer, row map[string]any) error {
	_, err := fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n",
		row["family"], row["repo"], row["key"], row["severity"], row["url"])
	return err
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

// emitSecurityWarnings writes the collector's per-repo access warnings to
// stderr. It is intentionally split from the exit-status check so callers
// can run it BEFORE rendering — a render-time failure (template parse,
// embedded-newline rejection) must not swallow the warnings, which the
// team-security spec requires to appear on stderr in every output mode.
func emitSecurityWarnings(c *cobra.Command, res *security.Result) {
	stderr := c.ErrOrStderr()
	for _, w := range res.Warnings {
		fmt.Fprintln(stderr, w)
	}
}

// securityExitStatus returns the typed exit-status error when the
// collector saw at least one hard access failure, or nil otherwise. The
// per-repo information is already on stderr via emitSecurityWarnings;
// this error only carries the non-zero exit signal.
func securityExitStatus(res *security.Result) error {
	if res.HardFailures > 0 {
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

