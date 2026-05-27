package security

import (
	"context"
	"fmt"
)

// codeScanningAlert mirrors the relevant subset of the
// /repos/{owner}/{repo}/code-scanning/alerts response.
type codeScanningAlert struct {
	State   string `json:"state"`
	HTMLURL string `json:"html_url"`
	Rule    struct {
		ID                    string `json:"id"`
		Severity              string `json:"severity"`
		SecuritySeverityLevel string `json:"security_severity_level"`
	} `json:"rule"`
}

func codeScanningInitialPath(owner, repo string) string {
	return fmt.Sprintf("repos/%s/%s/code-scanning/alerts?state=open&per_page=%d",
		owner, repo, pageSize)
}

// fetchCodeScanning follows Link-header pagination through every open
// code-scanning alert and applies the spec's severity rule: prefer
// security_severity_level, fall back to rule.severity.
func fetchCodeScanning(ctx context.Context, c Client, owner, repo string) ([]AlertRow, error) {
	full := owner + "/" + repo
	var out []AlertRow
	err := paginate(ctx, c, codeScanningInitialPath(owner, repo), func(body []byte) error {
		var batch []codeScanningAlert
		if err := decodeJSONArray(body, &batch); err != nil {
			return err
		}
		for _, a := range batch {
			if a.State != "" && a.State != "open" {
				continue
			}
			sev := a.Rule.SecuritySeverityLevel
			if sev == "" {
				sev = a.Rule.Severity
			}
			out = append(out, AlertRow{
				Family:   FamilyCodeScanning,
				Repo:     full,
				Key:      a.Rule.ID,
				Severity: sev,
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
