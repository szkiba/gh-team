// Package security collects open Dependabot and code-scanning alerts for a
// set of repositories using GitHub's repository-level REST endpoints. The
// collector is read-only, paginates each repository/family pair, and treats
// repositories where a feature is unavailable as contributing zero alerts.
package security

import (
	"fmt"
	"sort"
)

// Family names a single alert source supported by the collector.
type Family string

const (
	FamilyDependabot   Family = "dependabot"
	FamilyCodeScanning Family = "code-scanning"
)

// Kind is the user-facing --kind value. `all` is a fixed alias for the
// dependabot + code-scanning union (see design.md Decision 4).
type Kind string

const (
	KindAll          Kind = "all"
	KindDependabot   Kind = "dependabot"
	KindCodeScanning Kind = "code-scanning"
)

// SupportedKinds is the public list of accepted --kind values in canonical
// order, used both by ParseKind and by error wording.
var SupportedKinds = []string{string(KindDependabot), string(KindCodeScanning), string(KindAll)}

// ParseKind validates a raw --kind argument and returns the canonical Kind.
// Anything outside SupportedKinds is rejected so callers can map directly to
// a non-zero exit per spec.
func ParseKind(s string) (Kind, error) {
	switch Kind(s) {
	case KindAll, KindDependabot, KindCodeScanning:
		return Kind(s), nil
	default:
		return "", fmt.Errorf("invalid --kind %q: supported values are %s, %s, and %s",
			s, KindDependabot, KindCodeScanning, KindAll)
	}
}

// Families expands a Kind to the concrete alert families that must be
// queried. `all` is frozen to dependabot + code-scanning so a future binary
// learning a third family does not silently change automation behavior.
func (k Kind) Families() []Family {
	switch k {
	case KindDependabot:
		return []Family{FamilyDependabot}
	case KindCodeScanning:
		return []Family{FamilyCodeScanning}
	case KindAll:
		return []Family{FamilyDependabot, FamilyCodeScanning}
	}
	return nil
}

// SummaryRow is one line of `security summary` output. Counts of zero are
// dropped by the renderer.
type SummaryRow struct {
	Repo   string
	Family Family
	Count  int
}

// AlertRow is one line of `security alerts` output.
type AlertRow struct {
	Family   Family
	Repo     string
	Key      string
	Severity string
	URL      string
}

// Result is what Collector.Collect returns. Summary and Alerts are sorted
// deterministically. Warnings are stderr lines for the caller to emit.
// HardFailures > 0 means the command must exit non-zero after rendering.
type Result struct {
	Summary      []SummaryRow
	Alerts       []AlertRow
	Warnings     []string
	HardFailures int
}

// sortSummary orders by repo, then by family — matches the spec's sample
// output for `security summary`.
func sortSummary(rows []SummaryRow) {
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Repo != rows[j].Repo {
			return rows[i].Repo < rows[j].Repo
		}
		return rows[i].Family < rows[j].Family
	})
}

// sortAlerts orders by repo, family, key, then URL — required so identical
// inputs always produce identical output regardless of fanout scheduling.
func sortAlerts(rows []AlertRow) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.Repo != b.Repo {
			return a.Repo < b.Repo
		}
		if a.Family != b.Family {
			return a.Family < b.Family
		}
		if a.Key != b.Key {
			return a.Key < b.Key
		}
		return a.URL < b.URL
	})
}
