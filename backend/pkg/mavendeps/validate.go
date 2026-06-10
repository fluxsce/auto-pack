package mavendeps

import (
	"fmt"
	"strings"
)

// UnsatisfiedDependency is a compile-scope dependency that cannot be resolved for build.
type UnsatisfiedDependency struct {
	Dependency       Coordinates `json:"dependency"`
	Reason           string      `json:"reason"`
	WorkspaceProject string      `json:"workspace_project,omitempty"`
}

// ValidateTargetDependencies checks that every direct dependency of the target
// is available before the main build runs.
func (w *Workspace) ValidateTargetDependencies(targetRecID string) []UnsatisfiedDependency {
	if w == nil {
		return nil
	}
	targetRecID = strings.TrimSpace(targetRecID)
	target, ok := w.byRecID[targetRecID]
	if !ok {
		return []UnsatisfiedDependency{{
			Reason: fmt.Sprintf("target project not in workspace index: %s", targetRecID),
		}}
	}
	var out []UnsatisfiedDependency
	seen := map[string]struct{}{}
	for _, dep := range target.Dependencies {
		key := dep.Key() + "@" + dep.Version
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		if dep.GroupID == target.Coordinates.GroupID && dep.ArtifactID == target.Coordinates.ArtifactID {
			continue
		}
		if u := w.checkDependencySatisfied(target, dep); u != nil {
			out = append(out, *u)
		}
	}
	return out
}

func (w *Workspace) checkDependencySatisfied(consumer *Module, dep Coordinates) *UnsatisfiedDependency {
	if provider := w.matchModule(consumer, dep); provider != nil {
		coord := RequiredDependencyCoord(consumer, dep, provider)
		if w.workspaceArtifactSatisfied(consumer, dep, provider) {
			return nil
		}
		return &UnsatisfiedDependency{
			Dependency:       coord,
			WorkspaceProject: provider.ProjectName,
			Reason: fmt.Sprintf(
				"工作空间上游 %s (%s) 未安装到本地仓库，请先构建上游工程",
				provider.ProjectName, coord.Key()+"@"+coord.Version,
			),
		}
	}
	if cand := w.findWorkspaceModule(dep); cand != nil && cand.ProjectRecID != consumer.ProjectRecID {
		if w.workspaceUpstreamMatch(consumer, dep, cand) {
			coord := RequiredDependencyCoord(consumer, dep, cand)
			if w.workspaceArtifactSatisfied(consumer, dep, cand) {
				return nil
			}
			return &UnsatisfiedDependency{
				Dependency:       coord,
				WorkspaceProject: cand.ProjectName,
				Reason: fmt.Sprintf(
					"工作空间上游 %s (%s) 未安装到本地仓库，请先构建上游工程",
					cand.ProjectName, coord.Key()+"@"+coord.Version,
				),
			}
		}
		return &UnsatisfiedDependency{
			Dependency:       dep,
			WorkspaceProject: cand.ProjectName,
			Reason: fmt.Sprintf(
				"工作空间工程 %s 与依赖 %s 版本未对齐（POM 需求 %s，扫描版本 %s）",
				cand.ProjectName, dep.Key(), dep.Version, moduleReleaseVersion(cand),
			),
		}
	}
	coord := resolvedExternalCoord(consumer, dep)
	if w.hasWorkspaceGroup(dep.GroupID) {
		if len(localRepositoryCandidates(w.localRepo, w.mavenHome)) == 0 {
			return &UnsatisfiedDependency{
				Dependency: coord,
				Reason: fmt.Sprintf(
					"无法解析本地 Maven 仓库路径，无法确认依赖 %s 是否已安装",
					coord.Key()+"@"+coord.Version,
				),
			}
		}
		if !w.artifactSatisfied(coord) {
			if cand := w.findWorkspaceModule(dep); cand != nil {
				return &UnsatisfiedDependency{
					Dependency:       coord,
					WorkspaceProject: cand.ProjectName,
					Reason: fmt.Sprintf(
						"工作空间上游 %s (%s) 未安装到本地仓库，请先构建上游工程",
						cand.ProjectName, coord.Key()+"@"+coord.Version,
					),
				}
			}
			return &UnsatisfiedDependency{
				Dependency: coord,
				Reason: fmt.Sprintf(
					"依赖 %s 不在本地仓库，请先将对应工程加入工作空间并构建上游",
					coord.Key()+"@"+coord.Version,
				),
			}
		}
	}
	return nil
}

func (w *Workspace) hasWorkspaceGroup(groupID string) bool {
	groupID = strings.TrimSpace(groupID)
	if groupID == "" {
		return false
	}
	for i := range w.modules {
		if w.modules[i].Coordinates.GroupID == groupID {
			return true
		}
	}
	return false
}

func resolvedExternalCoord(consumer *Module, dep Coordinates) Coordinates {
	return Coordinates{
		GroupID:    dep.GroupID,
		ArtifactID: dep.ArtifactID,
		Version:    resolveDependencyVersion(consumer, dep),
	}
}
