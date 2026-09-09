package mod

import "testing"

func TestPath(t *testing.T) {
	if Path("jobs", "{id}", "log") != "/autopack/api/jobs/{id}/log" {
		t.Fatal(Path("jobs", "{id}", "log"))
	}
	if Path() != Root {
		t.Fatal(Path())
	}
}
