package git

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"auto-pack/backend/pkg/shell"
)

// versionTagLabelPattern matches project version tags like V9.0.0-P08 or V9.0.0-P01-01.
var versionTagLabelPattern = regexp.MustCompile(`^V[\d.]+-P\d+(-\d+)?$`)

// NormalizeTagName strips refs/tags/ prefixes and surrounding space.
func NormalizeTagName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "refs/tags/")
	s = strings.TrimPrefix(s, "tags/")
	s = strings.TrimPrefix(s, "tag/")
	return strings.TrimSpace(s)
}

// FormatTagRef returns the canonical repo_branch value for a version tag (refs/tags/NAME).
func FormatTagRef(tag string) string {
	name := NormalizeTagName(tag)
	if name == "" {
		return ""
	}
	return "refs/tags/" + name
}

// IsTagRef reports whether repoBranch stores a tag ref.
func IsTagRef(repoBranch string) bool {
	return strings.HasPrefix(strings.TrimSpace(repoBranch), "refs/tags/")
}

// IsVersionTagLabel reports whether s is a short version tag name (not a git branch path).
func IsVersionTagLabel(s string) bool {
	s = NormalizeTagName(s)
	if s == "" || strings.Contains(s, "/") {
		return false
	}
	return versionTagLabelPattern.MatchString(s)
}

// NormalizeRepoBranchForStorage returns canonical repo_branch: version tags become refs/tags/NAME.
func NormalizeRepoBranchForStorage(repoBranch string) string {
	repoBranch = strings.TrimSpace(repoBranch)
	if repoBranch == "" {
		return ""
	}
	if IsTagRef(repoBranch) {
		return FormatTagRef(repoBranch)
	}
	if strings.Contains(repoBranch, "/") {
		return repoBranch
	}
	if IsVersionTagLabel(repoBranch) {
		return FormatTagRef(repoBranch)
	}
	return repoBranch
}

// CheckoutRefCandidates returns refs to try with git checkout -f (newest preference first).
func CheckoutRefCandidates(repoBranch string) []string {
	repoBranch = strings.TrimSpace(repoBranch)
	if repoBranch == "" {
		return nil
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(repoBranch)
	if IsTagRef(repoBranch) {
		name := NormalizeTagName(repoBranch)
		add(name)
		add("tags/" + name)
	} else if IsVersionTagLabel(repoBranch) {
		add(FormatTagRef(repoBranch))
	}
	return out
}

// ValidateTagName checks that name is a valid one-level tag under refs/tags/.
func ValidateTagName(name string) error {
	name = NormalizeTagName(name)
	if name == "" {
		return fmt.Errorf("git tag name is empty")
	}
	exe, err := Executable()
	if err != nil {
		return err
	}
	ref := "refs/tags/" + name
	ctx := context.Background()
	res, err := shell.RunArgs(ctx, exe, []string{"check-ref-format", "--allow-onelevel", ref}, shell.Options{AllowNonZeroExit: true})
	if err != nil {
		return err
	}
	if res.ExitCode == 0 {
		return nil
	}
	msg := strings.TrimSpace(res.Stderr)
	if msg == "" {
		msg = "invalid git tag name"
	}
	return fmt.Errorf("%s: %q", msg, name)
}
