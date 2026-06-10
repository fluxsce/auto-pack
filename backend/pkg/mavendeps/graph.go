package mavendeps

import (
	"fmt"
	"sort"
	"strings"
)

func versionMatches(depVer, moduleVer, consumerPomVer string) bool {
	depVer = strings.TrimSpace(depVer)
	moduleVer = strings.TrimSpace(moduleVer)
	consumerPomVer = strings.TrimSpace(consumerPomVer)
	if depVer == "" {
		return true
	}
	if versionsEquivalent(depVer, moduleVer) {
		return true
	}
	if versionsEquivalent(depVer, consumerPomVer) {
		return true
	}
	return false
}

// versionsEquivalent compares Maven versions (exact or SNAPSHOT/release pair).
func versionsEquivalent(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return a == b
	}
	if strings.EqualFold(a, b) {
		return true
	}
	au := strings.TrimSuffix(strings.ToUpper(a), "-SNAPSHOT")
	bu := strings.TrimSuffix(strings.ToUpper(b), "-SNAPSHOT")
	return au != "" && au == bu
}

func isUnresolvedVersion(v string) bool {
	v = strings.TrimSpace(v)
	return strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}")
}

func isSnapshotVersion(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "" || strings.Contains(v, "snapshot")
}

// isLegacyFluxSnapshot is the placeholder pom version (e.g. 10.0.0-SNAPSHOT) before a V-release line.
func isLegacyFluxSnapshot(v string) bool {
	v = strings.TrimSpace(v)
	if v == "" || strings.HasPrefix(strings.ToUpper(v), "V") {
		return false
	}
	return isSnapshotVersion(v)
}

// modulePomVersion returns the effective Maven version from pom.xml (project_version is UI label only).
func modulePomVersion(m *Module) string {
	if m == nil {
		return ""
	}
	return strings.TrimSpace(m.Coordinates.Version)
}

func consumerReleaseVersion(consumer *Module) string {
	return modulePomVersion(consumer)
}

func moduleReleaseVersion(m *Module) string {
	return modulePomVersion(m)
}

func resolveDependencyVersion(consumer *Module, dep Coordinates) string {
	ver := strings.TrimSpace(dep.Version)
	if ver == "" || isUnresolvedVersion(ver) {
		return consumerReleaseVersion(consumer)
	}
	return ver
}

func artifactNamesMatch(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	return a != "" && b != "" && strings.EqualFold(a, b)
}

func fluxGroupIDAlternates(groupID string) []string {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" || !strings.HasPrefix(strings.ToLower(groupID), "com.flux.") {
		return nil
	}
	known := []string{"com.flux.scev6", "com.flux.wmsv6"}
	var out []string
	for _, alt := range known {
		if !strings.EqualFold(alt, groupID) {
			out = append(out, alt)
		}
	}
	return out
}

// groupIDsCompatible allows com.flux.wmsv6 deps to match modules whose parent POM uses com.flux.scev6.
func groupIDsCompatible(depGroup, moduleGroup string) bool {
	depGroup = strings.TrimSpace(depGroup)
	moduleGroup = strings.TrimSpace(moduleGroup)
	if depGroup == "" || moduleGroup == "" {
		return true
	}
	if strings.EqualFold(depGroup, moduleGroup) {
		return true
	}
	d := strings.ToLower(depGroup)
	m := strings.ToLower(moduleGroup)
	return strings.HasPrefix(d, "com.flux.") && strings.HasPrefix(m, "com.flux.")
}

func preferModuleForGAKey(candidate, current *Module) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	candExact := candidate.ProjectName == candidate.Coordinates.ArtifactID
	curExact := current.ProjectName == current.Coordinates.ArtifactID
	if candExact != curExact {
		return candExact
	}
	return false
}

func (w *Workspace) findCandidateByGA(dep Coordinates) *Module {
	return w.findWorkspaceModule(dep)
}

// findWorkspaceModule locates a workspace project for a Maven dependency coordinate.
func (w *Workspace) findWorkspaceModule(dep Coordinates) *Module {
	artifactID := strings.TrimSpace(dep.ArtifactID)
	if artifactID == "" {
		return nil
	}
	if m, ok := w.byKey[dep.Key()]; ok {
		return m
	}
	for i := range w.modules {
		m := &w.modules[i]
		if artifactNamesMatch(artifactID, m.Coordinates.ArtifactID) &&
			groupIDsCompatible(dep.GroupID, m.Coordinates.GroupID) {
			return m
		}
	}
	if m, ok := w.byName[artifactID]; ok {
		return m
	}
	if fold := strings.ToLower(artifactID); fold != artifactID {
		if m, ok := w.byName[fold]; ok {
			return m
		}
	}
	for i := range w.modules {
		m := &w.modules[i]
		if artifactNamesMatch(artifactID, m.ProjectName) {
			return m
		}
	}
	return nil
}

func (w *Workspace) sameWorkspaceArtifact(dep Coordinates, candidate *Module) bool {
	if candidate == nil {
		return false
	}
	if artifactNamesMatch(dep.ArtifactID, candidate.Coordinates.ArtifactID) &&
		groupIDsCompatible(dep.GroupID, candidate.Coordinates.GroupID) {
		return true
	}
	return artifactNamesMatch(dep.ArtifactID, candidate.ProjectName)
}

func (w *Workspace) moduleVersionMatches(consumer *Module, dep Coordinates, candidate *Module) bool {
	release := consumerReleaseVersion(consumer)
	if versionMatches(dep.Version, candidate.Coordinates.Version, release) {
		return true
	}
	if versionMatches(dep.Version, moduleReleaseVersion(candidate), release) {
		return true
	}
	depVer := strings.TrimSpace(dep.Version)
	moduleVer := moduleReleaseVersion(candidate)
	if depVer != "" && moduleVer != "" && depVer == moduleVer {
		return true
	}
	if (depVer == "" || isUnresolvedVersion(depVer)) && release != "" {
		if moduleVer == "" || isUnresolvedVersion(moduleVer) || isSnapshotVersion(moduleVer) || moduleVer == release {
			return true
		}
	}
	need := resolveDependencyVersion(consumer, dep)
	if need != "" && release != "" && need == release && w.sameWorkspaceArtifact(dep, candidate) {
		if isSnapshotVersion(candidate.Coordinates.Version) || isSnapshotVersion(moduleVer) {
			return true
		}
	}
	return false
}

func legacyUpstreamVersionMatch(consumer *Module, dep Coordinates, candidate *Module) bool {
	providerVer := moduleReleaseVersion(candidate)
	if !isLegacyFluxSnapshot(providerVer) {
		return false
	}
	need := resolveDependencyVersion(consumer, dep)
	if need != "" && versionsEquivalent(need, providerVer) {
		return true
	}
	release := consumerReleaseVersion(consumer)
	if release != "" && !versionsEquivalent(release, providerVer) {
		if need != "" && versionsEquivalent(need, release) {
			return true
		}
		// Consumer depends on V-release line while upstream pom still uses legacy snapshot placeholder.
		if need != "" && !versionsEquivalent(need, providerVer) {
			return true
		}
	}
	return false
}

func (w *Workspace) workspaceUpstreamMatch(consumer *Module, dep Coordinates, candidate *Module) bool {
	if consumer == nil || candidate == nil || candidate.ProjectRecID == consumer.ProjectRecID {
		return false
	}
	if !w.sameWorkspaceArtifact(dep, candidate) {
		return false
	}
	need := resolveDependencyVersion(consumer, dep)
	release := consumerReleaseVersion(consumer)
	if need != "" && release != "" && need == release {
		return true
	}
	if legacyUpstreamVersionMatch(consumer, dep, candidate) {
		return true
	}
	return w.moduleVersionMatches(consumer, dep, candidate)
}

func (w *Workspace) matchModule(consumer *Module, dep Coordinates) *Module {
	if consumer == nil {
		return nil
	}
	candidate := w.findWorkspaceModule(dep)
	if candidate == nil || candidate.ProjectRecID == consumer.ProjectRecID {
		return nil
	}
	if !w.workspaceUpstreamMatch(consumer, dep, candidate) {
		return nil
	}
	return candidate
}

// RequiredDependencyCoord is the GAV Maven will resolve for local-repo / upstream install checks.
func RequiredDependencyCoord(consumer *Module, dep Coordinates, provider *Module) Coordinates {
	ver := resolveDependencyVersion(consumer, dep)
	release := consumerReleaseVersion(consumer)
	providerVer := moduleReleaseVersion(provider)

	if ver == "" || isUnresolvedVersion(ver) {
		ver = release
	}
	// Upstream pom still on legacy 10.0.0-SNAPSHOT while consumer builds V10.0.0-P01-SNAPSHOT.
	if release != "" && providerVer != "" && ver == providerVer && !versionsEquivalent(ver, release) {
		ver = release
	}
	if ver == "" || isUnresolvedVersion(ver) {
		ver = providerVer
	}
	if ver == "" || isUnresolvedVersion(ver) {
		ver = release
	}
	artifactID := publishArtifactID(provider, dep)
	return Coordinates{
		GroupID:    strings.TrimSpace(provider.Coordinates.GroupID),
		ArtifactID: artifactID,
		Version:    ver,
	}
}

func publishArtifactID(provider *Module, dep Coordinates) string {
	if provider == nil {
		return ""
	}
	depArt := strings.TrimSpace(dep.ArtifactID)
	projName := strings.TrimSpace(provider.ProjectName)
	provArt := strings.TrimSpace(provider.Coordinates.ArtifactID)
	if depArt != "" && projName != "" && artifactNamesMatch(depArt, projName) {
		return depArt
	}
	if provArt != "" {
		return provArt
	}
	return projName
}

func (w *Workspace) workspaceArtifactSatisfied(consumer *Module, dep Coordinates, provider *Module) bool {
	if w == nil || provider == nil {
		return false
	}
	coord := RequiredDependencyCoord(consumer, dep, provider)
	return w.artifactSatisfied(coord)
}

// workspaceAdjacency builds provider -> consumers edges for internal modules only.
func (w *Workspace) workspaceAdjacency(needed map[string]bool, excludeRecID string) map[string][]string {
	adj := map[string][]string{}
	edgeSeen := map[string]struct{}{}
	for i := range w.modules {
		consumer := &w.modules[i]
		if !needed[consumer.ProjectRecID] {
			continue
		}
		if excludeRecID != "" && consumer.ProjectRecID == excludeRecID {
			continue
		}
		for _, dep := range consumer.Dependencies {
			provider := w.matchModule(consumer, dep)
			if provider == nil || provider.ProjectRecID == consumer.ProjectRecID {
				continue
			}
			if !needed[provider.ProjectRecID] {
				continue
			}
			if excludeRecID != "" && provider.ProjectRecID == excludeRecID {
				continue
			}
			key := provider.ProjectRecID + "->" + consumer.ProjectRecID
			if _, ok := edgeSeen[key]; ok {
				continue
			}
			edgeSeen[key] = struct{}{}
			adj[provider.ProjectRecID] = append(adj[provider.ProjectRecID], consumer.ProjectRecID)
		}
	}
	return adj
}

func (w *Workspace) topoSort(needed map[string]bool, excludeRecID string) ([]Module, error) {
	adj := w.workspaceAdjacency(needed, excludeRecID)
	inDegree := map[string]int{}
	for id := range needed {
		if excludeRecID != "" && id == excludeRecID {
			continue
		}
		inDegree[id] = 0
	}
	for _, consumers := range adj {
		for _, id := range consumers {
			inDegree[id]++
		}
	}
	queue := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}
	sort.Strings(queue)
	out := make([]Module, 0, len(inDegree))
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if m, ok := w.byRecID[id]; ok {
			out = append(out, *m)
		}
		next := append([]string(nil), adj[id]...)
		sort.Strings(next)
		for _, to := range next {
			inDegree[to]--
			if inDegree[to] == 0 {
				queue = append(queue, to)
				sort.Strings(queue)
			}
		}
	}
	if len(out) != len(inDegree) {
		return nil, fmt.Errorf("cyclic maven dependency between workspace projects")
	}
	return out, nil
}

func (w *Workspace) findCycleHint(needed map[string]bool, excludeRecID string) string {
	adj := w.workspaceAdjacency(needed, excludeRecID)
	visited := map[string]int{} // 0 unvisited, 1 visiting, 2 done
	stack := make([]string, 0)
	var cycle []string
	var dfs func(string) bool
	dfs = func(id string) bool {
		switch visited[id] {
		case 1:
			for i, n := range stack {
				if n == id {
					cycle = append(cycle, stack[i:]...)
					cycle = append(cycle, id)
					return true
				}
			}
			cycle = []string{id, id}
			return true
		case 2:
			return false
		}
		visited[id] = 1
		stack = append(stack, id)
		for _, to := range adj[id] {
			if dfs(to) {
				return true
			}
		}
		stack = stack[:len(stack)-1]
		visited[id] = 2
		return false
	}
	for id := range needed {
		if excludeRecID != "" && id == excludeRecID {
			continue
		}
		if visited[id] == 0 && dfs(id) {
			break
		}
	}
	if len(cycle) == 0 {
		return ""
	}
	names := make([]string, 0, len(cycle))
	for _, id := range cycle {
		if m, ok := w.byRecID[id]; ok {
			names = append(names, m.ProjectName)
		} else {
			names = append(names, id)
		}
	}
	return strings.Join(names, " → ")
}

func (w *Workspace) stableModuleOrder(needed map[string]bool, excludeRecID string) []Module {
	list := make([]Module, 0, len(needed))
	for i := range w.modules {
		m := w.modules[i]
		if !needed[m.ProjectRecID] {
			continue
		}
		if excludeRecID != "" && m.ProjectRecID == excludeRecID {
			continue
		}
		list = append(list, m)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ProjectName < list[j].ProjectName
	})
	return list
}
