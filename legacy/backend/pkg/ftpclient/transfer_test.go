package ftpclient

import "testing"

func TestJoinFTPPath(t *testing.T) {
	tests := []struct {
		base, rel, want string
	}{
		{"/", "war/a.txt", "/war/a.txt"},
		{"/", "/war/a.txt", "/war/a.txt"},
		{"/home", "war/a.txt", "/home/war/a.txt"},
		{"/home", "/war/datahub/sapjco/test1/Version.txt", "/war/datahub/sapjco/test1/Version.txt"},
		{"", "a/b", "/a/b"},
	}
	for _, tc := range tests {
		got := joinFTPPath(normalizeBase(tc.base), tc.rel)
		if got != tc.want {
			t.Fatalf("joinFTPPath(%q,%q)=%q want %q", tc.base, tc.rel, got, tc.want)
		}
	}
}
