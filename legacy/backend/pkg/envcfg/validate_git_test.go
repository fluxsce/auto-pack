package envcfg

import (
	"testing"

	"auto-pack/backend/pkg/envtypes"
)

func TestValidateGitHTTPS(t *testing.T) {
	if err := ValidateGit(GitConfig{AuthType: envtypes.GitAuthHTTPS, AuthToken: "tok"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGit(GitConfig{
		AuthType:     envtypes.GitAuthHTTPS,
		AuthUsername: "u",
		AuthPassword: "p",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGit(GitConfig{AuthType: envtypes.GitAuthHTTPS}); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateGitSSH(t *testing.T) {
	if err := ValidateGit(GitConfig{AuthType: envtypes.GitAuthSSH, PrivateKeyPath: "/keys/id_rsa"}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateGit(GitConfig{AuthType: envtypes.GitAuthSSH}); err == nil {
		t.Fatal("expected error")
	}
}
