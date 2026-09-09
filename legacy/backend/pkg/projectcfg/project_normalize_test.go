package projectcfg

import "testing"

func TestNormalizeProjectSCM(t *testing.T) {
	name, ver, url, branch, remark := NormalizeProjectSCM(
		"  demo  ", " 1.0.0 ", " https://git.example.com/a.git ", "  develop ", "  note ",
	)
	if name != "demo" || ver != "1.0.0" || url != "https://git.example.com/a.git" || branch != "develop" || remark != "note" {
		t.Fatalf("got %q %q %q %q %q", name, ver, url, branch, remark)
	}
	_, ver2, _, branch2, _ := NormalizeProjectSCM("x", "", "", "", "")
	if ver2 != "1.0.0" || branch2 != "main" {
		t.Fatalf("defaults: version=%q branch=%q", ver2, branch2)
	}
}
