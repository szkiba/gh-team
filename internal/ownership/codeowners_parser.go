package ownership

import "strings"

// teamOwnsWildcard reports whether @org/team is one of the owners on the
// LAST line whose first token is exactly "*" (the bare wildcard). This
// implements CODEOWNERS' last-matching-pattern precedence at the * scope.
//
// Comments (`#` to end-of-line) are stripped per line, blank lines are
// ignored, and the team-slug comparison is case-insensitive — matching
// GitHub's own behavior. Path-scoped patterns are ignored entirely.
func teamOwnsWildcard(content, org, team string) bool {
	target := strings.ToLower("@" + org + "/" + team)

	var lastOwners []string
	for _, raw := range strings.Split(content, "\n") {
		line := raw
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if fields[0] != "*" {
			continue
		}
		lastOwners = fields[1:]
	}

	for _, o := range lastOwners {
		if strings.EqualFold(o, target) {
			return true
		}
	}
	return false
}
