package notes

import (
	"fmt"
	"os"
	"strings"
)

// SyncOptions configures Version.txt generation from UpdateNotes.txt.
type SyncOptions struct {
	Tag         string
	ReleaseDate string // YYYY-MM-DD
	AllowRepeat bool
}

// BuildVersionContent reads UpdateNotes and merges into Version.txt content.
// Call publish.GitCommitter.SyncReleaseWorktree first so versionPath reflects
// the release branch / base tag, not develop cache.
func BuildVersionContent(versionPath, updateNotesPath string, opts SyncOptions) (string, error) {
	tag := strings.TrimSpace(opts.Tag)
	if tag == "" {
		return "", fmt.Errorf("tag is required")
	}
	releaseDate := strings.TrimSpace(opts.ReleaseDate)
	if releaseDate == "" {
		return "", fmt.Errorf("release_date is required")
	}
	releaseYMD := dateToYMD(releaseDate)

	existing, err := os.ReadFile(versionPath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read Version.txt: %w", err)
	}
	existingContent := string(existing)

	notesBytes, err := os.ReadFile(updateNotesPath)
	if err != nil {
		return "", fmt.Errorf("read UpdateNotes.txt: %w", err)
	}
	entries := ParseUpdateNotes(string(notesBytes))
	blocks := ParseVersionFile(existingContent)
	prevYMD := PreviousReleaseDateYMD(blocks, tag, opts.AllowRepeat)
	selected := SelectNotesForRelease(entries, prevYMD, releaseYMD)

	block := VersionBlock{
		Version:     tag,
		ReleaseDate: releaseDate,
		Summary:     NotesToSummaryLines(selected),
	}
	return MergeVersionFile(existingContent, block, tag, opts.AllowRepeat)
}
