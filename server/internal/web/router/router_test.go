package router

import (
	"testing"

	"auto-pack/server/internal/web/mod"
)

func TestRegisteredModules(t *testing.T) {
	_ = Handler(mod.Deps{})
	got := mod.Names()
	want := []string{"audit", "auth", "envs", "health", "jobs", "projects", "release", "settings", "users", "workspaces"}
	if len(got) != len(want) {
		t.Fatalf("modules=%v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("modules=%v", got)
		}
	}
}
