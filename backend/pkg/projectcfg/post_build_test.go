package projectcfg

import "testing"

func TestPostBuildAction_ShouldRun(t *testing.T) {
	tests := []struct {
		when    string
		success bool
		want    bool
	}{
		{"success", true, true},
		{"success", false, false},
		{"failure", false, true},
		{"failure", true, false},
		{"always", true, true},
		{"always", false, true},
		{"", true, true},
	}
	for _, tc := range tests {
		a := PostBuildAction{Enabled: true, When: tc.when}
		if got := a.ShouldRun(tc.success); got != tc.want {
			t.Fatalf("when=%q success=%v: got %v want %v", tc.when, tc.success, got, tc.want)
		}
	}
	disabled := PostBuildAction{Enabled: false, When: "always"}
	if disabled.ShouldRun(true) {
		t.Fatal("disabled action should not run")
	}
}

func TestEffectivePostBuildActions_legacyDeploy(t *testing.T) {
	actions := EffectivePostBuildActions(nil, "env-123")
	if len(actions) != 1 {
		t.Fatalf("want 1 action got %d", len(actions))
	}
	if actions[0].EnvRecID != "env-123" || actions[0].Type != PostBuildSSHTransfer {
		t.Fatalf("unexpected action: %+v", actions[0])
	}
}

func TestParseConfigJSON_postBuildActions(t *testing.T) {
	raw := `{"build_cmd":"go build","post_build_actions":[{"type":"ssh_exec","enabled":true,"when":"success","env_rec_id":"x","command":"echo hi"}]}`
	cfg, err := ParseConfigJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.PostBuildActions) != 1 || cfg.PostBuildActions[0].Command != "echo hi" {
		t.Fatalf("unexpected: %+v", cfg.PostBuildActions)
	}
}
