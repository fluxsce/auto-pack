package projectcfg

import "strings"

// NormalizeProjectSCM trims project name, version, Git URL/branch, and remark.
func NormalizeProjectSCM(name, version, repoURL, branch, remark string) (string, string, string, string, string) {
	name = strings.TrimSpace(name)
	version = strings.TrimSpace(version)
	repoURL = strings.TrimSpace(repoURL)
	branch = strings.TrimSpace(branch)
	remark = strings.TrimSpace(remark)
	if version == "" {
		version = "1.0.0"
	}
	if branch == "" {
		branch = "main"
	}
	return name, version, repoURL, branch, remark
}
