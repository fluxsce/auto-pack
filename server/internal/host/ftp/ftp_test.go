package ftp

import "testing"

func TestMapConfigAliases(t *testing.T) {
	cfg := MapConfig(map[string]any{
		"host":      "ftp.local",
		"port":      21,
		"username":  "u",
		"password":  "p",
		"base_path": "/pub",
	})
	if cfg.Host != "ftp.local" || cfg.Port != "21" || cfg.RemoteDir != "/pub" {
		t.Fatalf("%+v", cfg)
	}
}
