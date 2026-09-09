package projectcfg

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var slugSanitize = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

// SlugFromProjectName derives a path-safe directory segment from a display name.
func SlugFromProjectName(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}

	slug := slugSanitize.ReplaceAllString(strings.ReplaceAll(trimmed, " ", "-"), "")
	if slug == "" {
		hash := uint32(0)
		for _, r := range trimmed {
			hash = hash*31 + uint32(r)
		}
		slug = fmt.Sprintf("p%x", hash%1_000_000_000)
	}
	if slug == "" || !unicode.IsLetter(rune(slug[0])) {
		slug = "p" + slug
	}
	if len(slug) > 64 {
		slug = slug[:64]
	}
	return slug
}
