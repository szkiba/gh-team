package security

import (
	"context"
	"fmt"
)

// dependabotAlert mirrors the relevant subset of the
// /repos/{owner}/{repo}/dependabot/alerts response. We only project the
// fields needed for the summary count and alert-row key/severity/url; the
// rest is intentionally dropped.
type dependabotAlert struct {
	State                 string `json:"state"`
	HTMLURL               string `json:"html_url"`
	SecurityVulnerability struct {
		Severity string `json:"severity"`
		Package  struct {
			Ecosystem string `json:"ecosystem"`
			Name      string `json:"name"`
		} `json:"package"`
	} `json:"security_vulnerability"`
	Dependency struct {
		ManifestPath string `json:"manifest_path"`
	} `json:"dependency"`
}

// dependabotInitialPath is the first request URL for one repository's open
// Dependabot alerts. Subsequent pages come from the Link header.
func dependabotInitialPath(owner, repo string) string {
	return fmt.Sprintf("repos/%s/%s/dependabot/alerts?state=open&per_page=%d",
		owner, repo, pageSize)
}

// fetchDependabot follows Link-header pagination through every open
// Dependabot alert for one repo and projects each into an AlertRow using
// the spec's "<ecosystem>:<package>@<manifest-path>" key. Client-side state
// filtering is a safety net for any future endpoint variant that ignores
// the query.
func fetchDependabot(ctx context.Context, c Client, owner, repo string) ([]AlertRow, error) {
	full := owner + "/" + repo
	var out []AlertRow
	err := paginate(ctx, c, dependabotInitialPath(owner, repo), func(body []byte) error {
		var batch []dependabotAlert
		if err := decodeJSONArray(body, &batch); err != nil {
			return err
		}
		for _, a := range batch {
			if a.State != "" && a.State != "open" {
				continue
			}
			key := fmt.Sprintf("%s:%s@%s",
				a.SecurityVulnerability.Package.Ecosystem,
				a.SecurityVulnerability.Package.Name,
				a.Dependency.ManifestPath,
			)
			out = append(out, AlertRow{
				Family:   FamilyDependabot,
				Repo:     full,
				Key:      key,
				Severity: a.SecurityVulnerability.Severity,
				URL:      a.HTMLURL,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}
