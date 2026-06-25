package nodejs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInjectPnpmAllowAllBuilds(t *testing.T) {
	in := "npm install -g pnpm\npnpm  install\npnpm run build"
	got := InjectPnpmAllowAllBuilds(in)
	if !strings.Contains(got, "pnpm  install --dangerously-allow-all-builds") {
		t.Fatalf("missing flag on install: %q", got)
	}
	if strings.Contains(got, "dangerously-allow-all-builds run") {
		t.Fatalf("must not inject on pnpm run: %q", got)
	}
}

func TestAppendPnpmAllowAllBuildsEnv(t *testing.T) {
	env := AppendPnpmAllowAllBuildsEnv([]string{"PATH=/bin"})
	if !hasEnvKey(env, "npm_config_dangerously_allow_all_builds") {
		t.Fatalf("missing npm_config env: %v", env)
	}
}

func TestInjectPnpmAllowAllBuilds_idempotent(t *testing.T) {
	in := "pnpm install --dangerously-allow-all-builds"
	if got := InjectPnpmAllowAllBuilds(in); got != in {
		t.Fatalf("got %q", got)
	}
}

func TestEnsureWorkdirPnpmAllowBuilds_appends(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".npmrc"), []byte("registry=http://x/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := EnsureWorkdirPnpmAllowBuilds(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, ".npmrc"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "dangerously-allow-all-builds=true") {
		t.Fatalf("missing allow builds: %q", string(data))
	}
}
