package env

import (
	"testing"
)

func TestNormalizeKind(t *testing.T) {
	got, err := NormalizeKind("nodejs")
	if err != nil || got != "node" {
		t.Fatalf("nodejs -> %q %v", got, err)
	}
	if _, err := NormalizeKind("oracle"); err == nil {
		t.Fatal("unknown kind must fail")
	}
}

func TestNormalizeConfigFTPAlias(t *testing.T) {
	cfg, err := NormalizeConfig("ftp", map[string]any{
		"host":      "ftp.example.com",
		"port":      "21",
		"base_path": "/pub",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg["remote_dir"] != "/pub" {
		t.Fatalf("remote_dir=%v", cfg["remote_dir"])
	}
	if cfg["host"] != "ftp.example.com" {
		t.Fatalf("host=%v", cfg["host"])
	}
	if asString(cfg["port"]) != "21" {
		t.Fatalf("port=%v", cfg["port"])
	}
}

func TestRedactAndMergeSecrets(t *testing.T) {
	stored := map[string]any{"host": "h", "password": "secret"}
	public, set := Redact("ftp", stored)
	if public["password"] != nil {
		t.Fatal("password must not leak")
	}
	if public["host"] != "h" {
		t.Fatalf("host=%v", public["host"])
	}
	if len(set) != 1 || set[0] != "password" {
		t.Fatalf("secrets_set=%v", set)
	}
	incoming, err := NormalizeConfig("ftp", map[string]any{"host": "h2", "password": ""})
	if err != nil {
		t.Fatal(err)
	}
	merged := MergeSecrets("ftp", incoming, stored)
	if merged["password"] != "secret" {
		t.Fatalf("keep password, got %v", merged["password"])
	}
	if merged["host"] != "h2" {
		t.Fatalf("host=%v", merged["host"])
	}
}

func TestValidateConfigRequiresHost(t *testing.T) {
	cfg, err := NormalizeConfig("ssh", map[string]any{})
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateConfig("ssh", cfg); err == nil {
		t.Fatal("host required")
	}
	cfg["host"] = "10.0.0.1"
	if err := ValidateConfig("ssh", cfg); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeConfigSMTPTo(t *testing.T) {
	cfg, err := NormalizeConfig("smtp", map[string]any{
		"host": "mail.example",
		"to":   "ops@example.com,dev@example.com",
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg["to"] != "ops@example.com,dev@example.com" {
		t.Fatalf("to=%v", cfg["to"])
	}
}

func TestConfigHintUsesPublicField(t *testing.T) {
	h := ConfigHint("jdk", map[string]any{"java_home": "/opt/jdk-21"})
	if h == "" || h == "configured" {
		t.Fatalf("hint=%q", h)
	}
}
