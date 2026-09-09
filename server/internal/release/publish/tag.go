package publish

import (
	"strings"

	"auto-pack/server/internal/release/suggest"
)

func normalizeGitTagName(s string) string {
	return suggest.NormalizeVersionTag(s)
}

func gitTagNameFromReleaseBranch(releaseBranch string) string {
	releaseBranch = strings.TrimSpace(releaseBranch)
	for _, prefix := range []string{"release/", "releases/"} {
		if strings.HasPrefix(releaseBranch, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(releaseBranch, prefix))
		}
	}
	return ""
}

func gitTagRefCandidates(tagName string) []string {
	tagName = normalizeGitTagName(tagName)
	if tagName == "" {
		return nil
	}
	return dedupeOrdered([]string{
		"refs/tags/" + tagName,
		"tags/" + tagName,
		tagName,
	})
}

func resolveGitTagRefsForCheckout(releaseBranch, versionTag string) []string {
	var refs []string
	if name := gitTagNameFromReleaseBranch(releaseBranch); name != "" {
		refs = append(refs, gitTagRefCandidates(name)...)
	}
	if extra := normalizeGitTagName(versionTag); extra != "" {
		refs = append(refs, gitTagRefCandidates(extra)...)
	}
	return dedupeOrdered(refs)
}

func dedupeOrdered(items []string) []string {
	seen := make(map[string]struct{}, len(items))
	out := make([]string, 0, len(items))
	for _, s := range items {
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
