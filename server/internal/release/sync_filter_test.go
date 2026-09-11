package release

import (
	"testing"

	"auto-pack/server/internal/project"
)

func TestFilterProjectsByRecID(t *testing.T) {
	list := []project.Project{
		{RecID: "a", ProjectName: "A"},
		{RecID: "b", ProjectName: "B"},
		{RecID: "c", ProjectName: "C"},
	}
	if got := FilterProjectsByRecID(list, nil); len(got) != 3 {
		t.Fatalf("empty filter: %d", len(got))
	}
	got := FilterProjectsByRecID(list, []string{" c ", "a", "a", ""})
	if len(got) != 2 || got[0].RecID != "a" || got[1].RecID != "c" {
		t.Fatalf("%+v", got)
	}
}
