package suggest

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	tagPatchRevRe    = regexp.MustCompile(`^(.+-P\d+)-(\d+)$`)
	tagPatchOnlyRe   = regexp.MustCompile(`^(.+-P\d+)$`)
	tagPatchSuffixRe = regexp.MustCompile(`-P(\d+)$`)
)

// TagMatchesWorkspace reports whether tag belongs to the workspace code lineage.
func TagMatchesWorkspace(tag, workspaceCode string) bool {
	tag = normalizeGitTagName(tag)
	workspaceCode = strings.TrimSpace(workspaceCode)
	if tag == "" || workspaceCode == "" {
		return false
	}
	if tag == workspaceCode {
		return true
	}
	return strings.HasPrefix(tag, workspaceCode+"-")
}

type tagSortKey struct {
	raw   string
	patch int
	rev   int // 0 when no -NN revision suffix
}

func tagSortKeyFromName(tag string) tagSortKey {
	tag = normalizeGitTagName(tag)
	key := tagSortKey{raw: tag}
	if m := tagPatchRevRe.FindStringSubmatch(tag); len(m) == 3 {
		if patch, err := parsePatchNumber(m[1]); err == nil {
			key.patch = patch
		}
		if rev, err := strconv.Atoi(m[2]); err == nil {
			key.rev = rev
		}
		return key
	}
	if tagPatchOnlyRe.MatchString(tag) {
		if patch, err := parsePatchNumber(tag); err == nil {
			key.patch = patch
		}
	}
	return key
}

func parsePatchNumber(tag string) (int, error) {
	m := tagPatchSuffixRe.FindStringSubmatch(tag)
	if len(m) != 2 {
		return 0, fmt.Errorf("no -P segment")
	}
	return strconv.Atoi(m[1])
}

func compareTagKeys(a, b tagSortKey) int {
	if a.patch != b.patch {
		return a.patch - b.patch
	}
	if a.rev != b.rev {
		return a.rev - b.rev
	}
	return strings.Compare(a.raw, b.raw)
}

func maxMatchedTag(tags []string, workspaceCode string) string {
	var best tagSortKey
	found := false
	for _, t := range tags {
		t = normalizeGitTagName(t)
		if !TagMatchesWorkspace(t, workspaceCode) {
			continue
		}
		key := tagSortKeyFromName(t)
		if !found || compareTagKeys(key, best) > 0 {
			best = key
			found = true
		}
	}
	if !found {
		return ""
	}
	return best.raw
}

func incrementMatchedTag(tag, workspaceCode string) string {
	tag = normalizeGitTagName(tag)
	workspaceCode = strings.TrimSpace(workspaceCode)
	if tag == "" {
		return ""
	}
	if m := tagPatchRevRe.FindStringSubmatch(tag); len(m) == 3 {
		base, revStr := m[1], m[2]
		rev, _ := strconv.Atoi(revStr)
		width := len(revStr)
		if width < 2 {
			width = 2
		}
		return fmt.Sprintf("%s-%0*d", base, width, rev+1)
	}
	if tagPatchOnlyRe.MatchString(tag) {
		if tag == workspaceCode {
			return tag + "-01"
		}
		pm := tagPatchSuffixRe.FindStringSubmatch(tag)
		if len(pm) == 2 {
			patch, _ := strconv.Atoi(pm[1])
			width := len(pm[1])
			if width < 2 {
				width = 2
			}
			prefix := tag[:strings.LastIndex(tag, "-P")+2]
			return fmt.Sprintf("%s%0*d", prefix, width, patch+1)
		}
	}
	return tag + "-01"
}

// SuggestNextVersionTag picks the next tag after the max tag matching workspaceCode.
// When no prior tag exists, returns workspaceCode.
func SuggestNextVersionTag(workspaceCode string, knownTags []string) string {
	workspaceCode = strings.TrimSpace(workspaceCode)
	if workspaceCode == "" {
		return ""
	}
	max := maxMatchedTag(knownTags, workspaceCode)
	if max == "" {
		return workspaceCode
	}
	return incrementMatchedTag(max, workspaceCode)
}

// DefaultReleaseBranch returns release/{workspaceCode}.
func DefaultReleaseBranch(workspaceCode string) string {
	workspaceCode = strings.TrimSpace(workspaceCode)
	if workspaceCode == "" {
		return ""
	}
	return "release/" + workspaceCode
}
