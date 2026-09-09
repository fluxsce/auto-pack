package notes

import "testing"

func TestSelectNotesForRelease(t *testing.T) {
	entries := ParseUpdateNotes("20260409 zhangsan fix login\n20260301 lisi old\n")
	got := SelectNotesForRelease(entries, "20260315", "20260409")
	if len(got) != 1 || got[0].DateYMD != "20260409" {
		t.Fatalf("%+v", got)
	}
}
