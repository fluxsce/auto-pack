// Package suggest 根据工作空间编码与已有标签建议下一版本 Tag 与默认发版分支。
package suggest

import (
	"fmt"
	"strings"
)

// NormalizeVersionTag returns the canonical git tag label.
func NormalizeVersionTag(s string) string {
	return normalizeGitTagName(s)
}

func normalizeGitTagName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "refs/tags/")
	s = strings.TrimPrefix(s, "tags/")
	return strings.TrimSpace(s)
}

// ValidateVersionTag ensures the version tag satisfies git ref naming rules.
func ValidateVersionTag(tag string) error {
	tag = normalizeGitTagName(tag)
	if tag == "" {
		return fmt.Errorf("invalid version tag: empty")
	}
	if strings.Contains(tag, "..") || strings.Contains(tag, "//") {
		return fmt.Errorf("invalid version tag")
	}
	if strings.HasPrefix(tag, "/") || strings.HasSuffix(tag, "/") {
		return fmt.Errorf("invalid version tag")
	}
	if strings.ContainsAny(tag, " \t\n~^:?*[\\") {
		return fmt.Errorf("invalid version tag")
	}
	return nil
}

// RepoBranchAfterRelease picks the repo_branch to sync after a successful batch release.
func RepoBranchAfterRelease(releaseBranch, targetBranch string, useTarget bool) string {
	if useTarget {
		if b := strings.TrimSpace(targetBranch); b != "" {
			return b
		}
	}
	return strings.TrimSpace(releaseBranch)
}
