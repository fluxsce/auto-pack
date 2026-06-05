package git

import "testing"

func TestParseLsRemoteTags(t *testing.T) {
	stdout := `abc123 refs/tags/V9.0.0-P08
def456 refs/tags/V9.0.0-P08^{}
ghi789 refs/tags/V9.0.0-P08-01
`
	got := ParseLsRemoteTags(stdout)
	if len(got) != 2 || got[0] != "V9.0.0-P08" || got[1] != "V9.0.0-P08-01" {
		t.Fatalf("got %v", got)
	}
}
