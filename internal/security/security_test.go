package security

import "testing"

func TestParseKind(t *testing.T) {
	cases := []struct {
		in      string
		want    Kind
		wantErr bool
	}{
		{"all", KindAll, false},
		{"dependabot", KindDependabot, false},
		{"code-scanning", KindCodeScanning, false},
		{"secret-scanning", "", true},
		{"", "", true},
		{"ALL", "", true},
		{"dependabot,code-scanning", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseKind(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ParseKind(%q) err=%v, wantErr=%v", tc.in, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("ParseKind(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestKindAllIsFrozen locks Decision 4 from design.md: `all` must expand
// exactly to dependabot + code-scanning so adding a third family in a later
// change does not silently change command output.
func TestKindAllIsFrozen(t *testing.T) {
	got := KindAll.Families()
	want := []Family{FamilyDependabot, FamilyCodeScanning}
	if len(got) != len(want) {
		t.Fatalf("KindAll.Families() = %v, want %v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("KindAll.Families()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestParseKindErrorNamesSupported(t *testing.T) {
	_, err := ParseKind("secret-scanning")
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"dependabot", "code-scanning", "all"} {
		if !contains(msg, want) {
			t.Errorf("error %q missing supported value %q", msg, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
