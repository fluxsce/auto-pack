package projectrelease

import (
	"fmt"
	"strings"

	"auto-pack/backend/pkg/git"
)

// gitTagNamesForCheckout returns git tag names when creating releaseBranch from a tag.
// Uses only the tag implied by releaseBranch (release/V9.0.0-P01 -> V9.0.0-P01), not the Version.txt label.
func gitTagNamesForCheckout(releaseBranch, _ string) []string {
	if fromBranch := gitTagNameFromReleaseBranch(releaseBranch); fromBranch != "" {
		return []string{fromBranch}
	}
	return nil
}

// gitTagNameFromReleaseBranch maps release/V9.0.0-P01 -> V9.0.0-P01.
func gitTagNameFromReleaseBranch(releaseBranch string) string {
	releaseBranch = strings.TrimSpace(releaseBranch)
	for _, prefix := range []string{"release/", "releases/"} {
		if strings.HasPrefix(releaseBranch, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(releaseBranch, prefix))
		}
	}
	return ""
}

func normalizeGitTagName(s string) string {
	return git.NormalizeTagName(s)
}

// NormalizeVersionTag returns the canonical git tag label used for release and repo_branch sync.
func NormalizeVersionTag(s string) string {
	return normalizeGitTagName(s)
}

// RepoBranchForVersionTag returns refs/tags/NAME for storing in project.repo_branch.
func RepoBranchForVersionTag(tag string) string {
	return git.FormatTagRef(tag)
}

// ValidateVersionTag ensures the version tag satisfies git ref naming rules.
func ValidateVersionTag(tag string) error {
	tag = normalizeGitTagName(tag)
	if err := git.ValidateTagName(tag); err != nil {
		return fmt.Errorf("invalid version tag: %w", err)
	}
	return nil
}

// gitTagRefCandidates lists only git tag refs (not branches like origin/name).
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

// resolveGitTagRefsForCheckout builds ordered refs to create releaseBranch from tags.
func resolveGitTagRefsForCheckout(releaseBranch, versionTag string) []string {
	var refs []string
	for _, name := range gitTagNamesForCheckout(releaseBranch, versionTag) {
		refs = append(refs, gitTagRefCandidates(name)...)
	}
	return dedupeOrdered(refs)
}
