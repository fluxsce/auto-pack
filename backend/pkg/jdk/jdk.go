package jdk

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

// Verify checks JAVA_HOME / java binary and returns version output.
func Verify(ctx context.Context, cfg envtypes.JDKConfig) probe.Result {
	javaBin := resolveJavaBin(cfg)
	if javaBin == "" {
		return probe.Fail("java executable not configured", nil)
	}
	res, err := shell.RunArgs(ctx, javaBin, []string{"-version"}, shell.Options{})
	details := map[string]string{
		"java_path": javaBin,
	}
	if cfg.JavaHome != "" {
		details["java_home"] = cfg.JavaHome
	}
	combined := strings.TrimSpace(res.Stderr + "\n" + res.Stdout)
	if err != nil {
		if combined != "" {
			details["output"] = combined
		}
		return probe.Fail(fmt.Sprintf("java -version failed: %v", err), details)
	}
	if combined == "" {
		combined = "ok"
	}
	details["version"] = combined
	return probe.OK("JDK available", details)
}

func resolveJavaBin(cfg envtypes.JDKConfig) string {
	javaName := javaExecutableName()
	if p := strings.TrimSpace(cfg.JavaPath); p != "" {
		return normalizeJavaPath(p, javaName)
	}
	home := strings.TrimSpace(cfg.JavaHome)
	if home == "" {
		return ""
	}
	return filepath.Join(home, "bin", javaName)
}

func javaExecutableName() string {
	if os.PathSeparator == '\\' {
		return "java.exe"
	}
	return "java"
}

// normalizeJavaPath accepts a java binary, JAVA_HOME, or .../bin directory.
func normalizeJavaPath(p, javaName string) string {
	p = filepath.Clean(p)
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		if strings.EqualFold(filepath.Base(p), "bin") {
			return filepath.Join(p, javaName)
		}
		return filepath.Join(p, "bin", javaName)
	}
	if os.PathSeparator == '\\' && !strings.HasSuffix(strings.ToLower(p), ".exe") {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			if withExe := p + ".exe"; fileExists(withExe) {
				return withExe
			}
		}
	}
	return p
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
