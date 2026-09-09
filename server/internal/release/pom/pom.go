// Package pom 重写 Maven pom.xml 中的工程与已知 parent 版本。
package pom

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	pomFileName             = "pom.xml"
	datahubParentArtifactID = "datahub-utils-parentPom"
)

var (
	reParentVersion      = regexp.MustCompile(`(?s)(<version>)\s*[^<]*\s*(</version>)`)
	reParentArtifactID   = regexp.MustCompile(`(?s)<artifactId>\s*([^<]*?)\s*</artifactId>`)
	reVersionTag         = regexp.MustCompile(`<version>\s*([^<]*?)\s*</version>`)
	reParentBlock        = regexp.MustCompile(`(?s)<parent>.*?</parent>`)
	reDatahubCoreVersion = regexp.MustCompile(
		`(?s)(<datahub\.core\.version>)\s*[^<]*\s*(</datahub\.core\.version>)`,
	)
	rePropertiesOpen = regexp.MustCompile(`(?s)<properties>\s*`)
)

// projectSectionAnchors bound the project header; version tags below are never project version.
var projectSectionAnchors = []string{
	"<dependencies",
	"<dependencyManagement",
	"<modules",
	"<build",
	"<profiles",
	"<properties",
	"<distributionManagement",
	"<reporting",
	"<issueManagement",
	"<ciManagement",
}

// PomPath returns pom.xml path under the project work tree.
func PomPath(projectDir string) string {
	return filepath.Join(projectDir, pomFileName)
}

// PomVersionForTag returns the Maven version written into pom.xml.
// Git tags keep the product prefix (V9.0.0-P08); POM versions drop that V and append -SNAPSHOT.
func PomVersionForTag(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return ""
	}
	tag = stripLeadingVersionV(tag)
	if strings.HasSuffix(strings.ToUpper(tag), "-SNAPSHOT") {
		return tag
	}
	return tag + "-SNAPSHOT"
}

func stripLeadingVersionV(tag string) string {
	if len(tag) < 2 {
		return tag
	}
	if (tag[0] == 'V' || tag[0] == 'v') && tag[1] >= '0' && tag[1] <= '9' {
		return tag[1:]
	}
	return tag
}

// UpdatePomVersions sets project version, known parent versions, and optionally datahub.core.version to PomVersionForTag(tag).
func UpdatePomVersions(content, tag string) (string, error) {
	updated, skip, err := updatePomVersions(content, tag, "")
	if skip != "" {
		return "", fmt.Errorf("pom.xml: %s", skip)
	}
	return updated, err
}

// updatePomVersions returns skipReason when pom.xml is not applicable (caller should skip, not fail).
// projectParentArtifactID is the root aggregator artifactId; child module parent refs are updated when set.
func updatePomVersions(content, tag, projectParentArtifactID string) (updated string, skipReason string, err error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", "", fmt.Errorf("pom version tag is required")
	}
	if !strings.Contains(content, "<project") {
		return "", "无效的 pom.xml（缺少 project）", nil
	}
	pomVer := PomVersionForTag(tag)

	out := updateKnownParentVersions(content, pomVer, projectParentArtifactID)

	if rootOut, ok := tryReplaceRootProjectVersion(out, pomVer); ok {
		out = rootOut
	}
	if shouldSetDatahubCoreVersion(out) {
		out = upsertDatahubCoreVersion(out, pomVer)
	}
	if out == content {
		return "", "未找到可更新的版本号", nil
	}
	return out, "", nil
}

func updateKnownParentVersions(content, tag, projectParentArtifactID string) string {
	parentArtifactIDs := []string{datahubParentArtifactID}
	if id := strings.TrimSpace(projectParentArtifactID); id != "" {
		parentArtifactIDs = append(parentArtifactIDs, id)
	}
	return reParentBlock.ReplaceAllStringFunc(content, func(block string) string {
		for _, artifactID := range parentArtifactIDs {
			if !parentHasArtifactID(block, artifactID) {
				continue
			}
			if !reParentVersion.MatchString(block) {
				return block
			}
			return reParentVersion.ReplaceAllString(block, "${1}"+tag+"${2}")
		}
		return block
	})
}

func tryReplaceRootProjectVersion(content, tag string) (string, bool) {
	updated, err := replaceRootProjectVersion(content, tag)
	if err != nil {
		return "", false
	}
	return updated, true
}

// parseProjectArtifactID returns this pom's artifactId (not the parent block's).
func parseProjectArtifactID(content string) string {
	withoutParent := reParentBlock.ReplaceAllString(content, "")
	m := reParentArtifactID.FindStringSubmatch(withoutParent)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

// DiscoverPomPaths lists pom.xml files under a project tree (skips target/.git).
func DiscoverPomPaths(projectDir string) ([]string, error) {
	projectDir = filepath.Clean(strings.TrimSpace(projectDir))
	if projectDir == "" {
		return nil, fmt.Errorf("project directory is required")
	}
	var paths []string
	err := filepath.WalkDir(projectDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			switch d.Name() {
			case "target", ".git":
				return filepath.SkipDir
			}
			if strings.HasPrefix(d.Name(), ".") && path != projectDir {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == pomFileName {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	return paths, nil
}

// CollectPomVersionUpdates updates the root pom and every submodule pom under projectDir.
func CollectPomVersionUpdates(projectDir, tag string) (map[string][]byte, []string, error) {
	projectDir = strings.TrimSpace(projectDir)
	tag = strings.TrimSpace(tag)
	if projectDir == "" {
		return nil, nil, fmt.Errorf("project directory is required")
	}
	if tag == "" {
		return nil, nil, fmt.Errorf("pom version tag is required")
	}

	projectParentArtifactID := ""
	if raw, err := os.ReadFile(PomPath(projectDir)); err == nil {
		projectParentArtifactID = parseProjectArtifactID(string(raw))
	}

	paths, err := DiscoverPomPaths(projectDir)
	if err != nil {
		return nil, nil, err
	}
	if len(paths) == 0 {
		return nil, []string{"跳过 pom.xml 版本更新（未找到 pom.xml）"}, nil
	}

	pomVer := PomVersionForTag(tag)
	files := make(map[string][]byte, len(paths))
	var steps []string
	for _, pomPath := range paths {
		data, skipReason, err := readAndUpdatePomAt(pomPath, tag, projectParentArtifactID)
		if err != nil {
			return nil, steps, err
		}
		if skipReason != "" {
			rel, _ := filepath.Rel(projectDir, pomPath)
			if rel == "." {
				rel = pomFileName
			}
			steps = append(steps, fmt.Sprintf("跳过 %s 版本更新（%s）", rel, skipReason))
			continue
		}
		files[pomPath] = data
		rel, _ := filepath.Rel(projectDir, pomPath)
		if rel == "." {
			rel = pomFileName
		}
		// Memory only — disk is still unchanged until PublishChanges writes these bytes.
		steps = append(steps, fmt.Sprintf("已生成 %s 版本号 %s（版本 Tag %s，待写入工作区）", rel, pomVer, tag))
	}
	if len(files) == 0 {
		return nil, steps, nil
	}
	return files, steps, nil
}

func parentHasArtifactID(parentBlock, artifactID string) bool {
	artifactID = strings.TrimSpace(artifactID)
	if artifactID == "" {
		return false
	}
	m := reParentArtifactID.FindStringSubmatch(parentBlock)
	if len(m) < 2 {
		return false
	}
	return strings.TrimSpace(m[1]) == artifactID
}

func replaceRootProjectVersion(content, tag string) (string, error) {
	searchStart, searchEnd := projectVersionSearchBounds(content)
	searchRegion := content[searchStart:searchEnd]

	matches := reVersionTag.FindAllStringSubmatchIndex(searchRegion, -1)
	for _, loc := range matches {
		if len(loc) < 4 {
			continue
		}
		ver := strings.TrimSpace(searchRegion[loc[2]:loc[3]])
		if ver == "4.0.0" {
			continue
		}
		absStart := searchStart + loc[0]
		absEnd := searchStart + loc[1]
		if isInsideParent(content, absStart) {
			continue
		}
		return content[:absStart] + "<version>" + tag + "</version>" + content[absEnd:], nil
	}
	return "", fmt.Errorf("pom.xml: project version not found")
}

// projectVersionSearchBounds returns the slice where Maven project <version> may appear.
func projectVersionSearchBounds(content string) (start, end int) {
	start = 0
	if idx := strings.Index(strings.ToLower(content), "<modelversion>"); idx >= 0 {
		if close := strings.Index(strings.ToLower(content[idx:]), "</modelversion>"); close >= 0 {
			start = idx + close + len("</modelVersion>")
		}
	}
	end = len(content)
	lower := strings.ToLower(content)
	for _, anchor := range projectSectionAnchors {
		if idx := strings.Index(lower, anchor); idx >= start && idx < end {
			end = idx
		}
	}
	return start, end
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
	return readAndUpdatePomAt(pomPath, tag, "")
}

func readAndUpdatePomAt(pomPath, tag, projectParentArtifactID string) (data []byte, skipReason string, err error) {
	raw, err := os.ReadFile(pomPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "文件不存在", nil
		}
		return nil, "", fmt.Errorf("read pom.xml: %w", err)
	}
	updated, skip, err := updatePomVersions(string(raw), tag, projectParentArtifactID)
	if err != nil {
		return nil, "", err
	}
	if skip != "" {
		return nil, skip, nil
	}
	return []byte(updated), "", nil
}
