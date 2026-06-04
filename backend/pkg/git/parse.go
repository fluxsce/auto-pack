package git

import (
	"strconv"
	"strings"
)

func parsePorcelainStatus(raw string) WorktreeStatus {
	st := WorktreeStatus{
		RawPorcelain: raw,
		Clean:        true,
	}
	for _, line := range strings.Split(strings.TrimRight(raw, "\r\n"), "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "## ") {
			parseBranchLine(line[3:], &st)
			continue
		}
		if len(line) < 3 {
			continue
		}
		st.Clean = false
		xy := line[:2]
		path := strings.TrimSpace(line[3:])
		if i := strings.Index(path, " -> "); i >= 0 {
			path = path[i+4:]
		}
		switch {
		case strings.HasPrefix(xy, "??"):
			st.Untracked = append(st.Untracked, path)
		case strings.HasPrefix(xy, "!!"):
			st.Ignored = append(st.Ignored, path)
		case xy[0] == 'U' || xy[1] == 'U' || (xy[0] == 'A' && xy[1] == 'A'):
			st.Unmerged = append(st.Unmerged, path)
		case xy[0] == 'A' || xy[1] == 'A':
			st.Added = append(st.Added, path)
		case xy[0] == 'D' || xy[1] == 'D':
			st.Deleted = append(st.Deleted, path)
		case xy[0] == 'R' || xy[1] == 'R':
			st.Renamed = append(st.Renamed, path)
		case xy[0] == 'C' || xy[1] == 'C':
			st.Copied = append(st.Copied, path)
		default:
			st.Modified = append(st.Modified, path)
		}
	}
	return st
}

func parseBranchLine(body string, st *WorktreeStatus) {
	// main...origin/main [ahead 1, behind 2]
	// HEAD (no branch)
	detached := strings.HasPrefix(body, "HEAD")
	st.Detached = detached
	part := body
	if i := strings.Index(part, " "); i >= 0 {
		rest := part[i+1:]
		part = part[:i]
		parseTrackingExtras(rest, st)
	}
	if detached {
		return
	}
	if i := strings.Index(part, "..."); i >= 0 {
		st.Branch = part[:i]
		st.Upstream = part[i+3:]
		return
	}
	st.Branch = part
}

func parseTrackingExtras(rest string, st *WorktreeStatus) {
	rest = strings.TrimSpace(rest)
	if !strings.HasPrefix(rest, "[") || !strings.HasSuffix(rest, "]") {
		return
	}
	rest = strings.Trim(rest, "[]")
	for piece := range strings.SplitSeq(rest, ",") {
		piece = strings.TrimSpace(piece)
		fields := strings.Fields(piece)
		if len(fields) != 2 {
			continue
		}
		n, err := strconv.Atoi(fields[1])
		if err != nil {
			continue
		}
		switch fields[0] {
		case "ahead":
			st.Ahead = n
		case "behind":
			st.Behind = n
		}
	}
}
