package projectcfg

import "testing"

func TestValidateWorkspaceCode(t *testing.T) {
	valid := []string{"default", "prod", "dev_team", "Team-A", "has.dot", "team-a.prod", "env_v2"}
	for _, c := range valid {
		if err := ValidateWorkspaceCode(c); err != nil {
			t.Fatalf("%q should be valid: %v", c, err)
		}
	}
	invalid := []string{"", "1bad", "spa ce", "中文", "a/b", `a\b`, "a:b", "a..b", "..x", ".hidden"}
	for _, c := range invalid {
		if err := ValidateWorkspaceCode(c); err == nil {
			t.Fatalf("%q should be invalid", c)
		}
	}
}
