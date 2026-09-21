package order

import (
	"strings"

	"auto-pack/server/internal/project"
)

const (
	// OutcomeOK 工程构建成功。
	OutcomeOK = "ok"
	// OutcomeFailed 工程构建失败。
	OutcomeFailed = "failed"
	// OutcomeSkipped 因上游失败而跳过。
	OutcomeSkipped = "skipped"
)

// Gate 用分析图约束队列内启动顺序：只认 Queue 已收录的工程边，不丢上游补齐。
type Gate struct {
	providers map[string][]string
	names     map[string]string
}

// NewGate 从本次队列与 Maven/npm 分析边构造启动门闩。
// 只保留两端都在队列里的边；ExactTargets 未纳入的上游不会凭空拦住。
func NewGate(queue []project.Project, analysis MavenAnalysis) Gate {
	inQ := map[string]struct{}{}
	names := map[string]string{}
	for _, p := range queue {
		id := strings.TrimSpace(p.RecID)
		if id == "" {
			continue
		}
		inQ[id] = struct{}{}
		names[id] = p.ProjectName
	}
	providers := map[string][]string{}
	seen := map[string]struct{}{}
	for _, e := range analysis.InternalEdges {
		consumer := strings.TrimSpace(e.ConsumerRecID)
		provider := strings.TrimSpace(e.ProviderRecID)
		if consumer == "" || provider == "" || consumer == provider {
			continue
		}
		if _, ok := inQ[consumer]; !ok {
			continue
		}
		if _, ok := inQ[provider]; !ok {
			continue
		}
		key := consumer + ">" + provider
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		providers[consumer] = append(providers[consumer], provider)
	}
	return Gate{providers: providers, names: names}
}

// Decide 判断 id 能否启动。skipReason 非空表示上游失败/跳过，禁止开编。
// ready 表示队列内全部上游已成功（或没有队列内上游）。
func (g Gate) Decide(id string, done map[string]string) (ready bool, skipReason string) {
	id = strings.TrimSpace(id)
	var failed []string
	pending := 0
	for _, pid := range g.providers[id] {
		switch done[pid] {
		case OutcomeOK:
		case OutcomeFailed, OutcomeSkipped:
			if name := g.names[pid]; name != "" {
				failed = append(failed, name)
			} else {
				failed = append(failed, pid)
			}
		default:
			pending++
		}
	}
	if len(failed) > 0 {
		return false, "上游失败：" + strings.Join(failed, "、")
	}
	return pending == 0, ""
}

// ProviderNames 返回队列内上游工程名，供日志说明为何等待。
func (g Gate) ProviderNames(id string) []string {
	out := make([]string, 0, len(g.providers[id]))
	for _, pid := range g.providers[id] {
		if name := g.names[pid]; name != "" {
			out = append(out, name)
		}
	}
	return out
}
