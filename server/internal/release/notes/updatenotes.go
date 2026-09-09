package notes

import (
	"regexp"
	"strings"
)

var updateNoteDateLine = regexp.MustCompile(`^(\d{8})\s+(\S+)\s+(.*)$`)

// UpdateNoteEntry is one logical row from UpdateNotes.txt.
type UpdateNoteEntry struct {
	DateYMD string
	Author  string
	Content string
}

// ParseUpdateNotes reads changelog lines (newest first), merging continuation lines.
func ParseUpdateNotes(content string) []UpdateNoteEntry {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	lines := strings.Split(content, "\n")
	var entries []UpdateNoteEntry
	var cur *UpdateNoteEntry

	flush := func() {
		if cur == nil {
			return
		}
		cur.Content = strings.TrimSpace(cur.Content)
		if cur.Content != "" {
			entries = append(entries, *cur)
		}
		cur = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if isUpdateNotesSeparator(trimmed) {
			flush()
			continue
		}
		if m := updateNoteDateLine.FindStringSubmatch(trimmed); len(m) == 4 {
			flush()
			cur = &UpdateNoteEntry{
				DateYMD: m[1],
				Author:  m[2],
				Content: m[3],
			}
			continue
		}
		if cur != nil {
			if cur.Content != "" {
				cur.Content += "\n" + strings.TrimRight(line, "\r")
			} else {
				cur.Content = strings.TrimRight(line, "\r")
			}
		}
	}
	flush()
	return entries
}

func isUpdateNotesSeparator(line string) bool {
	if strings.HasPrefix(line, "====") || strings.HasPrefix(line, "----") {
		return true
	}
	if strings.HasPrefix(line, "================") {
		return true
	}
	return false
}

// SelectNotesForRelease returns notes with prevYMD < date <= releaseYMD (inclusive).
func SelectNotesForRelease(entries []UpdateNoteEntry, prevYMD, releaseYMD string) []UpdateNoteEntry {
	prevYMD = normalizeYMD(prevYMD)
	releaseYMD = normalizeYMD(releaseYMD)
	var out []UpdateNoteEntry
	for _, e := range entries {
		d := normalizeYMD(e.DateYMD)
		if d == "" {
			continue
		}
		if d > prevYMD && d <= releaseYMD {
			out = append(out, e)
		}
	}
	return out
}

func normalizeYMD(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "-", "")
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}

// NotesToSummaryLines converts selected entries to Version.txt summary lines.
func NotesToSummaryLines(entries []UpdateNoteEntry) []string {
	lines := make([]string, 0, len(entries))
	for _, e := range entries {
		content := strings.TrimSpace(e.Content)
		if content == "" {
			continue
		}
		for i, part := range strings.Split(content, "\n") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			if i == 0 {
				lines = append(lines, noteLinePrefix+part)
			} else {
				lines = append(lines, "\t   "+part)
			}
		}
	}
	return lines
}
