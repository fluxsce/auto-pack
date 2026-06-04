package projectrelease

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const pomFileName = "pom.xml"

var (
	reParentVersion      = regexp.MustCompile(`(?s)(<parent>\s*.*?\s*<version>)\s*[^<]*\s*(</version>)`)
	reVersionTag         = regexp.MustCompile(`<version>\s*([^<]*?)\s*</version>`)
	reParentBlock        = regexp.MustCompile(`(?s)<parent>.*?</parent>`)
	reDatahubCoreVersion = regexp.MustCompile(
		`(?s)(<datahub\.core\.version>)\s*[^<]*\s*(</datahub\.core\.version>)`,
	)
	rePropertiesOpen = regexp.MustCompile(`(?s)<properties>\s*`)
)

// PomPath returns pom.xml path under the project work tree.
func PomPath(projectDir string) string {
	return filepath.Join(projectDir, pomFileName)
}

// UpdatePomVersions sets project version, parent version (if present), and optionally datahub.core.version to tag.
func UpdatePomVersions(content, tag string) (string, error) {
	updated, skip, err := updatePomVersions(content, tag)
	if skip != "" {
		return "", fmt.Errorf("pom.xml: %s", skip)
	}
	return updated, err
}

// updatePomVersions returns skipReason when pom.xml is not applicable (caller should skip, not fail).
func updatePomVersions(content, tag string) (updated string, skipReason string, err error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", "", fmt.Errorf("pom version tag is required")
	}
	if !strings.Contains(content, "<project") {
		return "", "无效的 pom.xml（缺少 project）", nil
	}

	out := content
	if reParentVersion.MatchString(out) {
		out = reParentVersion.ReplaceAllString(out, "${1}"+tag+"${2}")
	}

	out, err = replaceRootProjectVersion(out, tag)
	if err != nil {
		return "", "未找到可更新的 project 版本号", nil
	}
	if shouldSetDatahubCoreVersion(out) {
		out = upsertDatahubCoreVersion(out, tag)
	}
	return out, "", nil
}

func replaceRootProjectVersion(content, tag string) (string, error) {
	matches := reVersionTag.FindAllStringSubmatchIndex(content, -1)
	for _, loc := range matches {
		if len(loc) < 4 {
			continue
		}
		ver := strings.TrimSpace(content[loc[2]:loc[3]])
		if ver == "4.0.0" {
			continue
		}
		if isInsideParent(content, loc[0]) {
			continue
		}
		updated := content[:loc[0]] + "<version>" + tag + "</version>" + content[loc[1]:]
		return updated, nil
	}
	return "", fmt.Errorf("pom.xml: project version not found")
}

func isInsideParent(content string, pos int) bool {
	for _, idx := range reParentBlock.FindAllStringIndex(content, -1) {
		if pos >= idx[0] && pos < idx[1] {
			return true
		}
	}
	return false
}

func shouldSetDatahubCoreVersion(content string) bool {
	if reDatahubCoreVersion.MatchString(content) {
		return true
	}
	lower := strings.ToLower(content)
	return strings.Contains(lower, "flux-collaboration-datahub")
}

func upsertDatahubCoreVersion(content, tag string) string {
	if reDatahubCoreVersion.MatchString(content) {
		return reDatahubCoreVersion.ReplaceAllString(content, "${1}"+tag+"${2}")
	}
	block := "\t<properties>\n\t\t<datahub.core.version>" + tag + "</datahub.core.version>\n\t</properties>\n\n"
	if rePropertiesOpen.MatchString(content) {
		return rePropertiesOpen.ReplaceAllStringFunc(content, func(m string) string {
			return m + "\t\t<datahub.core.version>" + tag + "</datahub.core.version>\n"
		})
	}
	for _, anchor := range []string{
		"\t<distributionManagement>",
		"\t<build>",
		"\t<dependencies>",
		"</project>",
	} {
		if idx := strings.Index(content, anchor); idx >= 0 {
			return content[:idx] + block + content[idx:]
		}
	}
	return strings.TrimRight(content, "\r\n") + "\n" + block
}

// ReadAndUpdatePom reads pom.xml and applies version updates.
func ReadAndUpdatePom(pomPath, tag string) ([]byte, error) {
	data, _, err := ReadAndUpdatePomOptional(pomPath, tag)
	return data, err
}

// ReadAndUpdatePomOptional skips unsupported pom.xml (skipReason set) instead of failing the release.
func ReadAndUpdatePomOptional(pomPath, tag string) (data []byte, skipReason string, err error) {
	raw, err := os.ReadFile(pomPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "文件不存在", nil
		}
		return nil, "", fmt.Errorf("read pom.xml: %w", err)
	}
	updated, skip, err := updatePomVersions(string(raw), tag)
	if err != nil {
		return nil, "", err
	}
	if skip != "" {
		return nil, skip, nil
	}
	return []byte(updated), "", nil
}
