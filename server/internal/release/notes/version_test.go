package notes

import "testing"

func TestMergeVersionFile(t *testing.T) {
	existing := "Version: V9.0.0-P08-01\n发布时间: 2026-03-15\n版本更新概要:\n\t   · old\n"
	out, err := MergeVersionFile(existing, VersionBlock{
		Version: "V9.0.0-P08-01", ReleaseDate: "2026-04-09", Summary: []string{"\t   · new"},
	}, "V9.0.0-P08-01", true)
	if err != nil {
		t.Fatal(err)
	}
	if ParseVersionFile(out)[0].ReleaseDate != "2026-04-09" {
		t.Fatal(out)
	}
}
