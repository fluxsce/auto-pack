package nodejs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"auto-pack/backend/pkg/envtypes"
)

func TestRegistryAuthKey(t *testing.T) {
	got := registryAuthKey("http://219.135.170.44:19081/repository/sce-dev-group/")
	if got != "//219.135.170.44:19081/repository/sce-dev-group/" {
		t.Fatalf("got %q", got)
	}
}

func TestRenderRegistryAuthBlock_token(t *testing.T) {
	block := renderRegistryAuthBlock(envtypes.NodeJSConfig{
		Registry:  "http://219.135.170.44:19081/repository/sce-dev-group/",
		AuthToken: "secret-token",
	})
	if !strings.Contains(block, "registry=") {
		t.Fatalf("missing registry: %q", block)
	}
	if !strings.Contains(block, ":_authToken=secret-token") {
		t.Fatalf("missing token: %q", block)
	}
	if !strings.Contains(block, ":always-auth=true") {
		t.Fatalf("missing always-auth: %q", block)
	}
}

func TestPrepareUserConfig_useExistingFile(t *testing.T) {
	dir := t.TempDir()
	npmrc := filepath.Join(dir, ".npmrc")
	if err := os.WriteFile(npmrc, []byte(`registry=http://example.com/npm/`), 0o644); err != nil {
		t.Fatal(err)
	}
	disallow := false
	path, cleanup, _, err := PrepareUserConfig(envtypes.NodeJSConfig{
		NpmrcPath:          npmrc,
		PnpmAllowAllBuilds: &disallow,
	}, dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if path != npmrc {
		t.Fatalf("got %q want %q", path, npmrc)
	}
}

func TestPrepareUserConfig_allowAllBuildsOnly(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	allow := true
	path, cleanup, summary, err := PrepareUserConfig(envtypes.NodeJSConfig{
		PnpmAllowAllBuilds: &allow,
	}, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if path == "" || !strings.Contains(summary, "allow all builds") {
		t.Fatalf("path=%q summary=%q", path, summary)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "dangerously-allow-all-builds=true") {
		t.Fatalf("missing allow-all-builds: %q", string(data))
	}
}

func TestPnpmAllowAllBuildsEnabled_defaultOn(t *testing.T) {
	if !PnpmAllowAllBuildsEnabled(envtypes.NodeJSConfig{}) {
		t.Fatal("expected default on when unset")
	}
	disallow := false
	if PnpmAllowAllBuildsEnabled(envtypes.NodeJSConfig{PnpmAllowAllBuilds: &disallow}) {
		t.Fatal("expected off when explicitly false")
	}
}

func TestPrepareUserConfig_mergeAuth(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	path, cleanup, summary, err := PrepareUserConfig(envtypes.NodeJSConfig{
		Registry:  "http://219.135.170.44:19081/repository/sce-dev-group/",
		AuthToken: "tok",
	}, dataDir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if path == "" || summary == "" {
		t.Fatal("expected temp npmrc")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, ":_authToken=tok") {
		t.Fatalf("missing auth in %q", content)
	}
}
