package ssh

import "testing"

func TestMapConfigAliases(t *testing.T) {
	cfg := MapConfig(map[string]any{
		"host":        "sftp.example",
		"port":        2222,
		"username":    "pack",
		"remote_dir":  "/opt/app",
		"timeout_sec": 30,
	})
	if cfg.Host != "sftp.example" || cfg.Port != "2222" || cfg.Username != "pack" {
		t.Fatalf("%+v", cfg)
	}
	if cfg.RemotePath != "/opt/app" {
		t.Fatal(cfg.RemotePath)
	}
	if cfg.Timeout.Seconds() != 30 {
		t.Fatal(cfg.Timeout)
	}
}

func TestDialRequiresHost(t *testing.T) {
	if _, err := Dial(Config{}); err == nil {
		t.Fatal("expected host required")
	}
}
