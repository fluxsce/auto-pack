package projectrelease

import (
	"fmt"
	"strings"
)

const (
	versionHeaderPrefix = "Version:"
	releaseDatePrefix   = "发布时间:"
	summaryHeader       = "版本更新概要:"
	noteLinePrefix      = "\t   · "
)

// VersionBlock is one version section in Version.txt.
type VersionBlock struct {
	Version     string
	ReleaseDate string // YYYY-MM-DD
	Summary     []string
}

// ParseVersionFile splits Version.txt into ordered blocks (newest first).
func ParseVersionFile(content string) []VersionBlock {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	var blocks []VersionBlock
	var cur *VersionBlock
	inSummary := false

	flush := func() {
		if cur == nil {
			return
		}
		blocks = append(blocks, *cur)
		cur = nil
		inSummary = false
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, versionHeaderPrefix) {
			flush()
			cur = &VersionBlock{
				Version: strings.TrimSpace(strings.TrimPrefix(trimmed, versionHeaderPrefix)),
			}
			inSummary = false
			continue
		}
		if cur == nil {
			continue
		}
		if strings.HasPrefix(trimmed, releaseDatePrefix) {
			cur.ReleaseDate = strings.TrimSpace(strings.TrimPrefix(trimmed, releaseDatePrefix))
			inSummary = false
			continue
		}
		if trimmed == summaryHeader || strings.HasPrefix(trimmed, "版本更新概要") {
			inSummary = true
			continue
		}
		if inSummary {
			if trimmed == "" {
				continue
			}
			cur.Summary = append(cur.Summary, line)
		}
	}
	flush()
	return blocks
}

// FormatVersionBlock renders a single block including trailing newline.
func FormatVersionBlock(b VersionBlock) string {
	var sb strings.Builder
	sb.WriteString(versionHeaderPrefix)
	sb.WriteString(" ")
	sb.WriteString(strings.TrimSpace(b.Version))
	sb.WriteByte('\n')
	sb.WriteString(releaseDatePrefix)
	sb.WriteString(" ")
	sb.WriteString(strings.TrimSpace(b.ReleaseDate))
	sb.WriteByte('\n')
	sb.WriteString(summaryHeader)
	sb.WriteByte('\n')
	for _, line := range b.Summary {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasPrefix(line, "\t") || strings.HasPrefix(line, " ") {
			sb.WriteString(line)
		} else {
			sb.WriteString(noteLinePrefix)
			sb.WriteString(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "·"), "·")))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// MergeVersionFile prepends or replaces the top block and keeps older blocks.
func MergeVersionFile(existing string, block VersionBlock, tag string, allowRepeat bool) (string, error) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", fmt.Errorf("version tag is required")
	}
	block.Version = tag
	blocks := ParseVersionFile(existing)
	out := make([]VersionBlock, 0, len(blocks)+1)
	if allowRepeat && len(blocks) > 0 && strings.EqualFold(strings.TrimSpace(blocks[0].Version), tag) {
		out = append(out, block)
		out = append(out, blocks[1:]...)
	} else {
		out = append(out, block)
		out = append(out, blocks...)
	}
	var sb strings.Builder
	for i, b := range out {
		sb.WriteString(FormatVersionBlock(b))
		if i < len(out)-1 {
			sb.WriteByte('\n')
		}
	}
	return strings.TrimRight(sb.String(), "\n") + "\n", nil
}

// PreviousReleaseDateYMD returns the cutoff date (YYYYMMDD) for UpdateNotes selection.
func PreviousReleaseDateYMD(blocks []VersionBlock, tag string, allowRepeat bool) string {
	if len(blocks) == 0 {
		return "00000000"
	}
	top := strings.TrimSpace(blocks[0].Version)
	tag = strings.TrimSpace(tag)
	if allowRepeat && strings.EqualFold(top, tag) {
		if len(blocks) > 1 {
			return dateToYMD(blocks[1].ReleaseDate)
		}
		return "00000000"
	}
	return dateToYMD(blocks[0].ReleaseDate)
}

func dateToYMD(date string) string {
	date = strings.TrimSpace(date)
	date = strings.ReplaceAll(date, "-", "")
	date = strings.ReplaceAll(date, "/", "")
	if len(date) >= 8 {
		return date[:8]
	}
	return "00000000"
}
