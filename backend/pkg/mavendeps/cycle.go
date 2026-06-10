package mavendeps

import (
	"fmt"
	"sort"
)

// CyclicGroup is a set of workspace modules with circular Maven dependencies.
type CyclicGroup struct {
	ProjectRecIDs []string `json:"project_rec_ids"`
	ProjectNames  []string `json:"project_names"`
}

func (w *Workspace) cycleAwareModuleOrder(needed map[string]bool, excludeRecID string) []Module {
	if order, err := w.topoSort(needed, excludeRecID); err == nil {
		return order
	}
	nodes := moduleIDsInSet(needed, excludeRecID)
	if len(nodes) == 0 {
		return nil
	}
	adj := w.workspaceAdjacency(needed, excludeRecID)
	sccs := tarjanSCC(nodes, adj)
	orderedSCCs := sccTopoOrder(sccs, adj)
	out := make([]Module, 0, len(nodes))
	for _, comp := range orderedSCCs {
		out = append(out, w.orderWithinSCC(comp, needed)...)
	}
	return out
}

func (w *Workspace) findCyclicGroups(needed map[string]bool, excludeRecID string) []CyclicGroup {
	nodes := moduleIDsInSet(needed, excludeRecID)
	if len(nodes) == 0 {
		return nil
	}
	adj := w.workspaceAdjacency(needed, excludeRecID)
	sccs := tarjanSCC(nodes, adj)
	var groups []CyclicGroup
	for _, comp := range sccs {
		if len(comp) < 2 {
			continue
		}
		sort.Strings(comp)
		g := CyclicGroup{ProjectRecIDs: append([]string(nil), comp...)}
		for _, id := range comp {
			if m, ok := w.byRecID[id]; ok {
				g.ProjectNames = append(g.ProjectNames, m.ProjectName)
			}
		}
		groups = append(groups, g)
	}
	sort.Slice(groups, func(i, j int) bool {
		if len(groups[i].ProjectNames) == 0 || len(groups[j].ProjectNames) == 0 {
			return groups[i].ProjectRecIDs[0] < groups[j].ProjectRecIDs[0]
		}
		return groups[i].ProjectNames[0] < groups[j].ProjectNames[0]
	})
	return groups
}

func moduleIDsInSet(needed map[string]bool, excludeRecID string) []string {
	out := make([]string, 0, len(needed))
	for id := range needed {
		if excludeRecID != "" && id == excludeRecID {
			continue
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

func tarjanSCC(nodes []string, adj map[string][]string) [][]string {
	index := 0
	stack := make([]string, 0)
	onStack := map[string]bool{}
	indices := map[string]int{}
	lowlink := map[string]int{}
	var sccs [][]string

	var strongConnect func(v string)
	strongConnect = func(v string) {
		indices[v] = index
		lowlink[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		for _, to := range adj[v] {
			if _, ok := indices[to]; !ok {
				strongConnect(to)
				if lowlink[to] < lowlink[v] {
					lowlink[v] = lowlink[to]
				}
			} else if onStack[to] {
				if indices[to] < lowlink[v] {
					lowlink[v] = indices[to]
				}
			}
		}

		if lowlink[v] == indices[v] {
			comp := make([]string, 0)
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				comp = append(comp, w)
				if w == v {
					break
				}
			}
			sccs = append(sccs, comp)
		}
	}

	for _, v := range nodes {
		if _, ok := indices[v]; !ok {
			strongConnect(v)
		}
	}
	return sccs
}

func sccTopoOrder(sccs [][]string, adj map[string][]string) [][]string {
	if len(sccs) == 0 {
		return nil
	}
	nodeSCC := map[string]int{}
	for i, comp := range sccs {
		for _, id := range comp {
			nodeSCC[id] = i
		}
	}
	sccAdj := map[int][]int{}
	inDegree := map[int]int{}
	for i := range sccs {
		inDegree[i] = 0
	}
	edgeSeen := map[string]struct{}{}
	for from, consumers := range adj {
		sFrom, okFrom := nodeSCC[from]
		if !okFrom {
			continue
		}
		for _, to := range consumers {
			sTo, okTo := nodeSCC[to]
			if !okTo || sFrom == sTo {
				continue
			}
			key := fmt.Sprintf("%d->%d", sFrom, sTo)
			if _, ok := edgeSeen[key]; ok {
				continue
			}
			edgeSeen[key] = struct{}{}
			sccAdj[sFrom] = append(sccAdj[sFrom], sTo)
			inDegree[sTo]++
		}
	}
	queue := make([]int, 0)
	for i, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, i)
		}
	}
	sort.Ints(queue)
	out := make([][]string, 0, len(sccs))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		out = append(out, append([]string(nil), sccs[id]...))
		next := append([]int(nil), sccAdj[id]...)
		sort.Ints(next)
		for _, to := range next {
			inDegree[to]--
			if inDegree[to] == 0 {
				queue = append(queue, to)
				sort.Ints(queue)
			}
		}
	}
	if len(out) != len(sccs) {
		// Fallback: append any missing components in stable order.
		seen := map[int]struct{}{}
		for _, comp := range out {
			if len(comp) == 0 {
				continue
			}
			seen[nodeSCC[comp[0]]] = struct{}{}
		}
		for i, comp := range sccs {
			if _, ok := seen[i]; ok {
				continue
			}
			out = append(out, append([]string(nil), comp...))
		}
	}
	return out
}

func (w *Workspace) orderWithinSCC(members []string, needed map[string]bool) []Module {
	memberSet := map[string]bool{}
	for _, id := range members {
		memberSet[id] = true
	}
	type scored struct {
		module    *Module
		installed bool
		inDegree  int
	}
	items := make([]scored, 0, len(members))
	for _, id := range members {
		m, ok := w.byRecID[id]
		if !ok || m == nil {
			continue
		}
		inDeg := 0
		for _, dep := range m.Dependencies {
			if provider := w.matchModule(m, dep); provider != nil && memberSet[provider.ProjectRecID] {
				inDeg++
			}
		}
		items = append(items, scored{
			module:    m,
			installed: w.artifactSatisfied(m.Coordinates),
			inDegree:  inDeg,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].installed != items[j].installed {
			return items[i].installed
		}
		if items[i].inDegree != items[j].inDegree {
			return items[i].inDegree < items[j].inDegree
		}
		return items[i].module.ProjectName < items[j].module.ProjectName
	})
	out := make([]Module, 0, len(items))
	for _, it := range items {
		out = append(out, *it.module)
	}
	return out
}

func cycleWarningSuffix(cyclicGroups []CyclicGroup) string {
	if len(cyclicGroups) == 0 {
		return "；已按强连通分量优化排序"
	}
	return "；已按强连通分量优化排序，自动构建时将对循环组进行多轮重试"
}
