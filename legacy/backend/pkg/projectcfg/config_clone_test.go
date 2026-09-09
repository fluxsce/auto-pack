package projectcfg

import "testing"

func TestRemapBuildDependsOn(t *testing.T) {
	raw := `{"build_triggers":{"depends_on":["old-a","old-b","external"],"schedule":{"enabled":false,"cron":""}}}`
	idMap := map[string]string{"old-a": "new-a", "old-b": "new-b"}
	out, err := RemapBuildDependsOn(raw, idMap)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := ParseConfigJSON(out)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.BuildTriggers.DependsOn) != 2 {
		t.Fatalf("depends_on len = %d, want 2: %v", len(cfg.BuildTriggers.DependsOn), cfg.BuildTriggers.DependsOn)
	}
	if cfg.BuildTriggers.DependsOn[0] != "new-a" || cfg.BuildTriggers.DependsOn[1] != "new-b" {
		t.Fatalf("unexpected depends_on: %v", cfg.BuildTriggers.DependsOn)
	}
}
