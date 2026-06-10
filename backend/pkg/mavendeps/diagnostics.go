package mavendeps

import "fmt"

// DependencyDiagnostics returns human-readable lines about workspace dependency resolution.
func (w *Workspace) DependencyDiagnostics(targetRecID string) []string {
	if w == nil {
		return nil
	}
	target, ok := w.byRecID[targetRecID]
	if !ok {
		return []string{fmt.Sprintf("Target not in workspace index: %s", targetRecID)}
	}
	lines := []string{fmt.Sprintf("Effective compile dependencies: %d", len(target.Dependencies))}
	for _, dep := range target.Dependencies {
		if provider := w.matchModule(target, dep); provider != nil {
			coord := RequiredDependencyCoord(target, dep, provider)
			if w.workspaceArtifactSatisfied(target, dep, provider) {
				lines = append(lines, fmt.Sprintf("  workspace upstream %s (%s): installed",
					provider.ProjectName, coord.Key()+"@"+coord.Version))
			} else {
				lines = append(lines, fmt.Sprintf("  workspace upstream %s (%s): missing in local repo",
					provider.ProjectName, coord.Key()+"@"+coord.Version))
			}
			continue
		}
		if cand := w.findWorkspaceModule(dep); cand != nil && cand.ProjectRecID != target.ProjectRecID {
			if !w.workspaceUpstreamMatch(target, dep, cand) {
				need := resolveDependencyVersion(target, dep)
				pomVer := modulePomVersion(cand)
				lines = append(lines, fmt.Sprintf("  %s version mismatch for %s (need %s, pom %s)",
					cand.ProjectName, dep.Key(), need, pomVer))
			}
		}
	}
	return lines
}
