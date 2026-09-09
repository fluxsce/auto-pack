package projectcfg

import "testing"

func TestApplyEmailNotifyToConfig_upsert(t *testing.T) {
	cfg := Config{
		BuildCmd: "mvn package",
		PostBuildActions: []PostBuildAction{{
			ID:   "old-id",
			Type: PostBuildSSHExec,
			Name: "deploy",
		}},
	}
	ApplyEmailNotifyToConfig(&cfg, EmailNotifyBulkOpts{
		Enabled: true, When: string(PostBuildWhenFailure),
		Recipients: "a@b.com", Subject: "fail", Body: "log",
	})
	if len(cfg.PostBuildActions) != 2 {
		t.Fatalf("actions len = %d", len(cfg.PostBuildActions))
	}
	var email *PostBuildAction
	for i := range cfg.PostBuildActions {
		if cfg.PostBuildActions[i].Type == PostBuildEmailNotify {
			email = &cfg.PostBuildActions[i]
		}
	}
	if email == nil {
		t.Fatal("email action missing")
	}
	if email.Recipients != "a@b.com" || !email.Enabled {
		t.Fatalf("unexpected email action: %+v", email)
	}

	ApplyEmailNotifyToConfig(&cfg, EmailNotifyBulkOpts{
		Enabled: false, Recipients: "x@y.com",
	})
	if len(cfg.PostBuildActions) != 2 {
		t.Fatalf("expected upsert not append, len=%d", len(cfg.PostBuildActions))
	}
	for i := range cfg.PostBuildActions {
		if cfg.PostBuildActions[i].Type == PostBuildEmailNotify {
			if cfg.PostBuildActions[i].ID != email.ID {
				t.Fatal("id changed on upsert")
			}
			if cfg.PostBuildActions[i].Enabled {
				t.Fatal("expected disabled")
			}
		}
	}
}
