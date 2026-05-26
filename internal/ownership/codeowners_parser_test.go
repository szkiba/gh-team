package ownership

import "testing"

func TestTeamOwnsWildcard(t *testing.T) {
	cases := []struct {
		name    string
		content string
		org     string
		team    string
		want    bool
	}{
		{"single wildcard owned", "* @octo/platform\n", "octo", "platform", true},
		{"multi-owner wildcard (team first)", "* @octo/platform @octo/security\n", "octo", "platform", true},
		{"multi-owner wildcard (team last)", "* @octo/security @octo/platform\n", "octo", "platform", true},
		{"earlier wildcard superseded by later", "* @octo/platform\n* @octo/other\n", "octo", "platform", false},
		{"later wildcard names team", "* @octo/other\n* @octo/platform\n", "octo", "platform", true},
		{"path-scoped only does not establish ownership", "/docs/ @octo/platform\n", "octo", "platform", false},
		{"team only in comment", "# fallback was * @octo/platform\n* @octo/other\n", "octo", "platform", false},
		{"trailing comment stripped after pattern", "* @octo/platform # legacy fallback\n", "octo", "platform", true},
		{"case-insensitive slug match", "* @OCTO/Platform\n", "octo", "platform", true},
		{"blank lines ignored", "\n\n* @octo/platform\n\n", "octo", "platform", true},
		{"empty content", "", "octo", "platform", false},
		{"wildcard present but team absent", "* @octo/other\n", "octo", "platform", false},
		{"path-scoped line after wildcard does not override", "* @octo/platform\n/docs/ @octo/other\n", "octo", "platform", true},
		{"tab-separated owners", "*\t@octo/platform\t@octo/other\n", "octo", "platform", true},
		{"no newline at end of file", "* @octo/platform", "octo", "platform", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := teamOwnsWildcard(tc.content, tc.org, tc.team)
			if got != tc.want {
				t.Errorf("teamOwnsWildcard(%q, %q, %q) = %v, want %v",
					tc.content, tc.org, tc.team, got, tc.want)
			}
		})
	}
}
