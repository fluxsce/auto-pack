package maven

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/backend/pkg/envtypes"
	"auto-pack/backend/pkg/probe"
	"auto-pack/backend/pkg/shell"
)

// Verify checks mvn binary and version.
func Verify(ctx context.Context, cfg envtypes.MavenConfig) probe.Result {
	mvnBin := resolveMvn(cfg)
	if mvnBin == "" {
		return probe.Fail("mvn executable not configured", nil)
	}
	opts := shell.Options{}
	if home := strings.TrimSpace(cfg.JavaHome); home != "" {
		opts.Env = []string{"JAVA_HOME=" + home}
	}
	res, err := shell.RunArgs(ctx, mvnBin, []string{"-version"}, opts)
	details := map[string]string{"mvn_path": mvnBin}
	if cfg.MavenHome != "" {
		details["maven_home"] = cfg.MavenHome
	}
	combined := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
	if err != nil {
		if combined != "" {
			details["output"] = combined
		}
		return probe.Fail(fmt.Sprintf("mvn -version failed: %v", err), details)
	}
	details["version"] = combined
	if settings := strings.TrimSpace(cfg.SettingsXML); settings != "" {
		resolved := ResolveSettingsPath(settings)
		if resolved == "" {
			return probe.Fail("settings.xml not found: "+settings, details)
		}
		details["settings_xml"] = resolved
	}
	return probe.OK("Maven available", details)
}

func resolveMvn(cfg envtypes.MavenConfig) string {
	if p := strings.TrimSpace(cfg.MvnPath); p != "" {
		return p
	}
	home := strings.TrimSpace(cfg.MavenHome)
	if home == "" {
		return ""
	}
	name := "mvn"
	if os.PathSeparator == '\\' {
		name = "mvn.cmd"
	}
	return filepath.Join(home, "bin", name)
}
