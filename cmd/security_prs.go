package cmd

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/spf13/cobra"

	"github.com/szkiba/gh-team/internal/security"
)

func newSecurityPrsCmd(flags *globalFlags) *cobra.Command {
	var titleFlag string
	var labelFlag []string
	out := &outputFlags{}
	c := &cobra.Command{
		Use:   "prs <org/team-slug>",
		Short: "List open pull requests in owned repositories that match security signals",
		Long: `Resolve the repositories owned by the team and print one open
pull request per line whose title or label set matches a security
signal.

Defaults (combined as OR):
  - title regex: (?i)^\[security\]|^security:|\[security\]$
  - label:       security

Use --title to replace the title regex (Go syntax, must compile). Use
--label to replace the label default; --label is repeatable.

Default mode emits one pull-request URL per line, so the output can
be piped into commands like 'xargs -I{} gh pr view {} --web' (per-
line invocation; plain "xargs gh pr view" batches multiple URLs into
one call and fails, since gh pr view accepts a single argument).
Output is sorted by repository ascending, then by pull-request
number descending (newest within each repo).

Use --json for a JSON array of pull-request objects, or --template
with a Go text/template to render one custom line per PR. The two
flags are mutually exclusive. JSON and template field names: .repo,
.number, .state, .title, .author, .updated, .url. JSON mode
preserves the original title verbatim.

Use --header to switch default mode to a labeled seven-column TSV
("repo\tnumber\tstate\ttitle\tauthor\tupdated\turl") prefixed by a
header line for spreadsheet import. Embedded tab and newline
characters in the title are replaced with a single space in
--header mode. --header is rejected with --json or --template.

Only PRs in state "open" are listed in v1. Listing pull requests on
private repositories requires repository-read access on the host gh
session (classic OAuth scope "repo", or an equivalent fine-grained
"Pull requests: read" permission) in addition to "read:org".`,
		Example: `  gh team security prs octo/platform
  gh team security prs octo/platform --label compliance --label audit
  gh team security prs octo/platform --title '^SEC-[0-9]+'
  gh team security prs octo/platform --json
  gh team security prs octo/platform --template '{{.repo}} {{.number}} {{.title}}'
  gh team security prs octo/platform --header`,
		Args: cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			return runSecurityPrs(c, flags, out, args[0], titleFlag, labelFlag)
		},
	}
	c.Flags().StringVar(&titleFlag, "title", "",
		"replace the default PR title regex (Go syntax); applied OR with --label")
	c.Flags().StringSliceVar(&labelFlag, "label", nil,
		"replace the default PR label match (repeatable); applied OR with --title")
	out.attach(c)
	return c
}

// buildPullMatcher applies the override-replaces-default semantics from the
// team-security spec: a user-supplied --title replaces the default title
// regex, a user-supplied --label list replaces the default label, and
// defaults only kick in when their respective flag is unset (or, for
// --title, set to the empty string).
func buildPullMatcher(titleFlag string, labelFlag []string) (*security.PullMatcher, error) {
	var titleRe *regexp.Regexp
	if titleFlag == "" {
		titleRe = security.DefaultPullTitleRegex()
	} else {
		re, err := regexp.Compile(titleFlag)
		if err != nil {
			return nil, fmt.Errorf("invalid --title %q: %w", titleFlag, err)
		}
		titleRe = re
	}
	labels := labelFlag
	if labels == nil {
		labels = []string{security.DefaultPullLabel}
	}
	return &security.PullMatcher{Title: titleRe, Labels: labels}, nil
}

func runSecurityPrs(c *cobra.Command, flags *globalFlags, out *outputFlags, arg, titleFlag string, labelFlag []string) error {
	plan, err := out.resolve()
	if err != nil {
		return err
	}
	matcher, err := buildPullMatcher(titleFlag, labelFlag)
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
	col := &security.PullsCollector{Client: client, Matcher: matcher}
	res, err := col.Collect(c.Context(), repos)
	if err != nil {
		return translateAPIError(err)
	}

	emitPullsWarnings(c, res)
	if err := plan.render(c.OutOrStdout(), pullRows(res.PullRequests), renderConfig{
		header:      "repo\tnumber\tstate\ttitle\tauthor\tupdated\turl",
		defFn:       renderPullDefault,
		defHeaderFn: renderPullWithHeaderColumns,
	}); err != nil {
		return err
	}
	return pullsExitStatus(res)
}

// pullRows projects collector PR rows into the public output contract.
// `.title` is preserved verbatim so JSON mode never silently sanitizes
// titles — default-mode rendering applies the sanitization at the
// renderPullDefault step, where the TSV one-line-per-row invariant
// actually matters.
func pullRows(rows []security.PullRequest) []map[string]any {
	out := make([]map[string]any, 0, len(rows))
	for _, r := range rows {
		out = append(out, map[string]any{
			"repo":    r.Repo,
			"number":  r.Number,
			"state":   r.State,
			"title":   r.Title,
			"author":  r.Author,
			"updated": r.Updated,
			"url":     r.URL,
		})
	}
	return out
}

// sanitizeTitleCell strips tab and newline characters from a PR title so a
// default-mode row stays single-line and the seven-column shape holds. The
// JSON path does NOT go through this helper.
func sanitizeTitleCell(s string) string {
	s = strings.ReplaceAll(s, "\t", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// renderPullDefault is the no-flag default: one PR URL per line.
func renderPullDefault(out io.Writer, row map[string]any) error {
	_, err := fmt.Fprintf(out, "%s\n", row["url"])
	return err
}

// renderPullWithHeaderColumns is the --header row formatter: the seven-column
// TSV that matches the header line. Sanitization keeps each row to a single
// line so the column shape holds.
func renderPullWithHeaderColumns(out io.Writer, row map[string]any) error {
	_, err := fmt.Fprintf(out, "%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
		row["repo"], row["number"], row["state"],
		sanitizeTitleCell(fmt.Sprint(row["title"])),
		row["author"], row["updated"], row["url"])
	return err
}

// emitPullsWarnings mirrors emitSecurityWarnings for the pulls collector so
// per-repo diagnostics reach stderr before rendering — a render-time
// failure (template parse error) must not swallow them.
func emitPullsWarnings(c *cobra.Command, res *security.PullsResult) {
	stderr := c.ErrOrStderr()
	for _, w := range res.Warnings {
		fmt.Fprintln(stderr, w)
	}
}

func pullsExitStatus(res *security.PullsResult) error {
	if res.HardFailures > 0 {
		return errSecurityIncomplete{count: res.HardFailures}
	}
	return nil
}
