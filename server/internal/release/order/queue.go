package order

import (
	"fmt"
	"sort"
	"strings"

	"auto-pack/server/internal/project"
)

// IsOrderedType reports whether the project participates in Maven / npm topology.
func IsOrderedType(projectType string) bool {
	switch strings.ToLower(strings.TrimSpace(projectType)) {
	case "maven", "npm", "node", "nodejs", "vue", "frontend":
		return true
	default:
		return false
	}
}

// QueueOpts selects which enabled projects enter an auto-build queue.
type QueueOpts struct {
	TargetRecIDs  []string
	IncludeOthers bool
	// ExactTargets keeps TargetRecIDs as-is (no pom.xml upstream expansion).
	ExactTargets bool
}

// Queue returns enabled projects in dependency order, then leftovers, then optional non-graph projects.
// skipped 为因工作空间上游已停用而未入队的下游说明（可直接写构建日志）。
func Queue(list []project.Project, analysis MavenAnalysis, opts QueueOpts) (out []project.Project, skipped []string) {
	enabled := make([]project.Project, 0, len(list))
	byID := map[string]project.Project{}
	disabled := map[string]project.Project{}
	for _, p := range list {
		if p.IsEnabled == 0 {
			disabled[p.RecID] = p
			continue
		}
		enabled = append(enabled, p)
		byID[p.RecID] = p
	}
	targets := map[string]struct{}{}
	for _, id := range opts.TargetRecIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			targets[id] = struct{}{}
		}
	}
	needed := map[string]struct{}{}
	if len(targets) == 0 {
		for _, m := range analysis.FullBuildOrder {
			needed[m.ProjectRecID] = struct{}{}
		}
		for _, p := range enabled {
			if IsOrderedType(p.ProjectType) {
				needed[p.RecID] = struct{}{}
			}
		}
	} else {
		for id := range targets {
			needed[id] = struct{}{}
		}
		if !opts.ExactTargets {
			collectUpstream(analysis.InternalEdges, needed)
		}
	}

	blocked, skipMsgs := consumersBlockedByDisabled(analysis.InternalEdges, needed, disabled, byID)
	skipped = skipMsgs
	for id := range blocked {
		delete(needed, id)
	}

	out = make([]project.Project, 0, len(needed))
	seen := map[string]struct{}{}
	for _, m := range analysis.FullBuildOrder {
		if _, ok := needed[m.ProjectRecID]; !ok {
			continue
		}
		if _, ok := seen[m.ProjectRecID]; ok {
			continue
		}
		p, ok := byID[m.ProjectRecID]
		if !ok {
			continue
		}
		out = append(out, p)
		seen[p.RecID] = struct{}{}
	}
	leftover := make([]project.Project, 0)
	for id := range needed {
		if _, ok := seen[id]; ok {
			continue
		}
		if p, ok := byID[id]; ok {
			leftover = append(leftover, p)
		}
	}
	sort.Slice(leftover, func(i, j int) bool { return leftover[i].ProjectName < leftover[j].ProjectName })
	for _, p := range leftover {
		out = append(out, p)
		seen[p.RecID] = struct{}{}
	}
	if len(targets) == 0 && opts.IncludeOthers {
		extras := make([]project.Project, 0)
		for _, p := range enabled {
			if _, ok := seen[p.RecID]; ok {
				continue
			}
			if IsOrderedType(p.ProjectType) {
				continue
			}
			extras = append(extras, p)
		}
		sort.Slice(extras, func(i, j int) bool { return extras[i].ProjectName < extras[j].ProjectName })
		out = append(out, extras...)
	}
	return out, skipped
}

// consumersBlockedByDisabled 若 needed 中消费者的上游（含传递）已停用，则跳过该消费者。
func consumersBlockedByDisabled(
	edges []MavenEdge,
	needed map[string]struct{},
	disabled map[string]project.Project,
	enabledByID map[string]project.Project,
) (blocked map[string]struct{}, msgs []string) {
	blocked = map[string]struct{}{}
	if len(disabled) == 0 {
		return blocked, nil
	}
	grew := true
	for grew {
		grew = false
		for _, e := range edges {
			if _, ok := needed[e.ConsumerRecID]; !ok {
				continue
			}
			if _, ok := blocked[e.ConsumerRecID]; ok {
				continue
			}
			_, provDisabled := disabled[e.ProviderRecID]
			_, provBlocked := blocked[e.ProviderRecID]
			if !provDisabled && !provBlocked {
				continue
			}
			blocked[e.ConsumerRecID] = struct{}{}
			grew = true
			consumerName := e.ConsumerName
			if p, ok := enabledByID[e.ConsumerRecID]; ok {
				consumerName = p.ProjectName
			}
			providerName := e.ProviderName
			if p, ok := disabled[e.ProviderRecID]; ok {
				providerName = p.ProjectName
			} else if p, ok := enabledByID[e.ProviderRecID]; ok {
				providerName = p.ProjectName
			}
			reason := "已停用"
			if provBlocked && !provDisabled {
				reason = "因上游停用而被跳过"
			}
			msgs = append(msgs, fmt.Sprintf("跳过 %s：上游 %s %s", consumerName, providerName, reason))
		}
	}
	sort.Strings(msgs)
	return blocked, msgs
}

func collectUpstream(edges []MavenEdge, needed map[string]struct{}) {
	grew := true
	for grew {
		grew = false
		for _, e := range edges {
			if _, ok := needed[e.ConsumerRecID]; !ok {
				continue
			}
			if _, ok := needed[e.ProviderRecID]; ok {
				continue
			}
			needed[e.ProviderRecID] = struct{}{}
			grew = true
		}
	}
}

// CyclicRecIDs returns project ids that sit in a cycle group.
func CyclicRecIDs(analysis MavenAnalysis) map[string]struct{} {
	out := map[string]struct{}{}
	for _, g := range analysis.CyclicGroups {
		for _, id := range g.ProjectRecIDs {
			if id = strings.TrimSpace(id); id != "" {
				out[id] = struct{}{}
			}
		}
	}
	return out
}
