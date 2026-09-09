// Package maven 改 POM、跑 mvn（含 dependency:tree）。无 HTTP、无权限表。
package maven

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"auto-pack/server/internal/host/bash"
)

var (
	reProjectVersion = regexp.MustCompile(`(?s)(<project[^>]*>.*?<version>)\s*[^<]*\s*(</version>)`)
	reParentVersion  = regexp.MustCompile(`(?s)(<parent>.*?<version>)\s*[^<]*\s*(</version>)`)
)

// UpdateFile sets project (and parent if present) version in pom.xml.
func UpdateFile(path, version string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out, err := Update(string(raw), version)
	if err != nil {
		return err
	}
	return os.WriteFile(path, []byte(out), 0o644)
}

// Update rewrites project version; parent version is updated when present.
func Update(content, version string) (string, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return "", fmt.Errorf("maven: version is required")
	}
	if !strings.Contains(content, "<project") {
		return "", fmt.Errorf("maven: invalid pom.xml")
	}
	if !reProjectVersion.MatchString(content) {
		return "", fmt.Errorf("maven: project version not found")
	}
	out := reProjectVersion.ReplaceAllString(content, "${1}"+version+"${2}")
	if reParentVersion.MatchString(out) {
		out = reParentVersion.ReplaceAllString(out, "${1}"+version+"${2}")
	}
	return out, nil
}

// Snapshot appends -SNAPSHOT when missing.
func Snapshot(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToUpper(tag), "-SNAPSHOT") {
		return tag
	}
	return tag + "-SNAPSHOT"
}

// Run executes a Maven command (default package, skip tests).
func Run(ctx context.Context, dir, command string, env []string, w io.Writer) error {
	if strings.TrimSpace(command) == "" {
		command = "mvn -B -DskipTests package"
	}
	return bash.Run(ctx, dir, command, env, w)
}
