package notes

import "strings"

// DefaultReleaseNote is the Git commit message when the user leaves release note empty.
func DefaultReleaseNote(tag string) string {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "auto-pack release"
	}
	return tag + "自动发版说明"
}

// ResolveCommitMessage returns the Git commit message for a release.
func ResolveCommitMessage(releaseNote, tag string) string {
	if msg := strings.TrimSpace(releaseNote); msg != "" {
		return msg
	}
	return DefaultReleaseNote(tag)
}
