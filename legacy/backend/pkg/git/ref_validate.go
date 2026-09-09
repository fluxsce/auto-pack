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

// NormalizeTagName strips refs/tags/, tags/, tag/ prefixes (repeated) and surrounding space.
func NormalizeTagName(s string) string {
	s = strings.TrimSpace(s)
	for {
		before := s
		s = strings.TrimPrefix(s, "refs/tags/")
		s = strings.TrimPrefix(s, "tags/")
		s = strings.TrimPrefix(s, "tag/")
		s = strings.TrimSpace(s)
		s = strings.TrimPrefix(s, "/")
		s = strings.TrimSpace(s)
		if s == before {
			break
		}
	}
	return s
}

// FormatTagRef returns the canonical repo_branch value for a version tag (refs/tags/NAME).
func FormatTagRef(tag string) string {
	name := NormalizeTagName(tag)
	if name == "" {
		return ""
	}
	return "refs/tags/" + name
}

// IsTagsPathRef reports tags/NAME shorthand (not refs/tags/).
func IsTagsPathRef(repoBranch string) bool {
	repoBranch = strings.TrimSpace(repoBranch)
	return strings.HasPrefix(repoBranch, "tags/") && !strings.HasPrefix(repoBranch, "refs/tags/")
}

// IsTagRef reports whether repoBranch uses an explicit tags path (tags/ or refs/tags/).
// Bare version labels like V9.0.0-P06 are treated as branch names, not tags.
func IsTagRef(repoBranch string) bool {
	repoBranch = strings.TrimSpace(repoBranch)
	if repoBranch == "" {
		return false
	}
	if strings.HasPrefix(repoBranch, "refs/tags/") {
		name := NormalizeTagName(repoBranch)
		return name != "" && !strings.Contains(name, "/")
	}
	if IsTagsPathRef(repoBranch) {
		name := NormalizeTagName(repoBranch)
		return name != "" && !strings.Contains(name, "/")
	}
	return false
}

// IsVersionTagLabel reports whether s is a short version tag name (not a git branch path).
func IsVersionTagLabel(s string) bool {
	s = NormalizeTagName(s)
	if s == "" || strings.Contains(s, "/") {
		return false
	}
	return versionTagLabelPattern.MatchString(s)
}

// NormalizeRepoBranchForStorage returns canonical repo_branch: tag refs become refs/tags/NAME.
func NormalizeRepoBranchForStorage(repoBranch string) string {
	repoBranch = strings.TrimSpace(repoBranch)
	if repoBranch == "" {
		return ""
	}
	if IsTagRef(repoBranch) {
		return FormatTagRef(repoBranch)
	}
	return repoBranch
}

// CloneBranchForSync returns git clone --branch value: tag refs use bare tag name (V9.0.0-P06).
func CloneBranchForSync(repoBranch string) string {
	if IsTagRef(repoBranch) {
		return NormalizeTagName(repoBranch)
	}
	return strings.TrimSpace(repoBranch)
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
	if IsTagRef(repoBranch) {
		name := NormalizeTagName(repoBranch)
		add(FormatTagRef(name))
		add(name)
		add("tags/" + name)
		return out
	}
	add(repoBranch)
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
