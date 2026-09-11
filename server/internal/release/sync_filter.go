package release

import (
	"strings"

	"auto-pack/server/internal/project"
)

// FilterProjectsByRecID 按 rec_id 子集裁剪工程列表；only 为空则原样返回。
func FilterProjectsByRecID(list []project.Project, only []string) []project.Project {
	if len(only) == 0 {
		return list
	}
	allow := make(map[string]struct{}, len(only))
	for _, id := range only {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		allow[id] = struct{}{}
	}
	if len(allow) == 0 {
		return list
	}
	out := make([]project.Project, 0, len(allow))
	for _, p := range list {
		if _, ok := allow[p.RecID]; ok {
			out = append(out, p)
		}
	}
	return out
}
