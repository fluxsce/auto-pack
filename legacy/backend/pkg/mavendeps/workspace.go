package mavendeps

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Module links a workspace project to its parsed pom.xml.
type Module struct {
	ProjectRecID   string        `json:"project_rec_id"`
	ProjectName    string        `json:"project_name"`
	ProjectVersion string        `json:"project_version"`
	PomPath        string        `json:"pom_path"`
	Coordinates    Coordinates   `json:"coordinates"`
	Dependencies   []Coordinates `json:"dependencies,omitempty"`
}

// AnalysisResult is the dependency graph outcome for one target project.
type AnalysisResult struct {
	TargetRecID      string        `json:"target_rec_id"`
	BuildOrder       []Module      `json:"build_order"`
	SkippedInstalled []Module      `json:"skipped_installed,omitempty"`
	CyclicGroups     []CyclicGroup `json:"cyclic_groups,omitempty"`
	MissingPOM       []string      `json:"missing_pom,omitempty"`
	Warnings         []string      `json:"warnings,omitempty"`
}

// InternalEdge is a workspace-internal Maven dependency (provider must build before consumer).
type InternalEdge struct {
	ConsumerRecID string      `json:"consumer_rec_id"`
	ConsumerName  string      `json:"consumer_name"`
	ProviderRecID string      `json:"provider_rec_id"`
	ProviderName  string      `json:"provider_name"`
	Dependency    Coordinates `json:"dependency"`
}

// WorkspaceAnalysisResult is the full workspace build order and dependency graph.
type WorkspaceAnalysisResult struct {
	ModuleCount      int            `json:"module_count"`
	FullBuildOrder   []Module       `json:"full_build_order"`
	BuildOrder       []Module       `json:"build_order"`
	SkippedInstalled []Module       `json:"skipped_installed,omitempty"`
	InternalEdges    []InternalEdge `json:"internal_edges,omitempty"`
	CyclicGroups     []CyclicGroup  `json:"cyclic_groups,omitempty"`
	MissingPOM       []string       `json:"missing_pom,omitempty"`
	Warnings         []string       `json:"warnings,omitempty"`
}

// Workspace indexes Maven modules in one workspace.
type Workspace struct {
	modules   []Module
	byKey     map[string]*Module
	byRecID   map[string]*Module
	byName    map[string]*Module
	localRepo string
	mavenHome string
}

// Modules returns all indexed workspace modules.
func (w *Workspace) Modules() []Module {
	if w == nil {
		return nil
	}
	out := make([]Module, len(w.modules))
	copy(out, w.modules)
	return out
}

// NewWorkspace builds the module index from scanned entries.
func NewWorkspace(modules []Module, localRepo string) *Workspace {
	return NewWorkspaceWithMavenHome(modules, localRepo, "")
}

// NewWorkspaceWithMavenHome builds the module index and optional MAVEN_HOME for repo checks.
func NewWorkspaceWithMavenHome(modules []Module, localRepo, mavenHome string) *Workspace {
	w := &Workspace{
		modules:   modules,
		byKey:     map[string]*Module{},
		byRecID:   map[string]*Module{},
		byName:    map[string]*Module{},
		localRepo: strings.TrimSpace(localRepo),
		mavenHome: strings.TrimSpace(mavenHome),
	}
	for i := range modules {
		m := &modules[i]
		w.byRecID[m.ProjectRecID] = m
		key := m.Coordinates.Key()
		if prev, ok := w.byKey[key]; ok {
			if preferModuleForGAKey(m, prev) {
				w.byKey[key] = m
			}
		} else {
			w.byKey[key] = m
		}
		if name := strings.TrimSpace(m.ProjectName); name != "" {
			if _, ok := w.byName[name]; !ok {
				w.byName[name] = m
			}
			if fold := strings.ToLower(name); fold != name {
				if prev, ok := w.byName[fold]; !ok || preferModuleForGAKey(m, prev) {
					w.byName[fold] = m
				}
			}
		}
	}
	return w
}

func (w *Workspace) artifactSatisfied(coord Coordinates) bool {
	for _, repo := range localRepositoryCandidates(w.localRepo, w.mavenHome) {
		if artifactInstalledAt(repo, coord) {
			return true
		}
	}
	return false
}

// AnalyzeTarget returns upstream modules in safe build order (dependencies first).
func (w *Workspace) AnalyzeTarget(targetRecID string) (*AnalysisResult, error) {
	targetRecID = strings.TrimSpace(targetRecID)
	target, ok := w.byRecID[targetRecID]
	if !ok {
		return nil, fmt.Errorf("target project not in workspace index: %s", targetRecID)
	}
	needed := map[string]bool{}
	requiredCoord := map[string]Coordinates{}
	var collect func(*Module)
	collect = func(m *Module) {
		for _, dep := range m.Dependencies {
			up := w.matchModule(m, dep)
			if up == nil || up.ProjectRecID == m.ProjectRecID {
				continue
			}
			coord := RequiredDependencyCoord(m, dep, up)
			if prev, ok := requiredCoord[up.ProjectRecID]; !ok || prev.Version != coord.Version {
				requiredCoord[up.ProjectRecID] = coord
			}
			if needed[up.ProjectRecID] {
				continue
			}
			needed[up.ProjectRecID] = true
			collect(up)
		}
	}
	collect(target)

	order, err := w.topoSort(needed, targetRecID)
	res := &AnalysisResult{TargetRecID: targetRecID}
	if err != nil {
		res.CyclicGroups = w.findCyclicGroups(needed, targetRecID)
		if hint := w.findCycleHint(needed, targetRecID); hint != "" {
			res.Warnings = append(res.Warnings,
				"检测到工作空间内 Maven 工程循环依赖: "+hint+cycleWarningSuffix(res.CyclicGroups))
		} else {
			res.Warnings = append(res.Warnings, err.Error())
		}
		order = w.cycleAwareModuleOrder(needed, targetRecID)
	}
	for _, m := range order {
		if req, ok := requiredCoord[m.ProjectRecID]; ok {
			if w.artifactSatisfied(req) {
				res.SkippedInstalled = append(res.SkippedInstalled, m)
				continue
			}
			res.BuildOrder = append(res.BuildOrder, m)
			continue
		}
		if w.artifactSatisfied(m.Coordinates) {
			res.SkippedInstalled = append(res.SkippedInstalled, m)
			continue
		}
		res.BuildOrder = append(res.BuildOrder, m)
	}
	return res, nil
}

// AnalyzeAll returns topological build order for every indexed module in the workspace.
func (w *Workspace) AnalyzeAll() (*WorkspaceAnalysisResult, error) {
	res := &WorkspaceAnalysisResult{ModuleCount: len(w.modules)}
	if len(w.modules) == 0 {
		return res, nil
	}
	needed := map[string]bool{}
	for i := range w.modules {
		needed[w.modules[i].ProjectRecID] = true
	}
	order, err := w.topoSort(needed, "")
	if err != nil {
		res.CyclicGroups = w.findCyclicGroups(needed, "")
		if hint := w.findCycleHint(needed, ""); hint != "" {
			res.Warnings = append(res.Warnings,
				"检测到工作空间内 Maven 工程循环依赖: "+hint+cycleWarningSuffix(res.CyclicGroups))
		} else {
			res.Warnings = append(res.Warnings, err.Error())
		}
		order = w.cycleAwareModuleOrder(needed, "")
	}
	res.InternalEdges = w.collectInternalEdges()
	res.FullBuildOrder = append(res.FullBuildOrder, order...)
	for _, m := range order {
		coord := m.Coordinates
		if w.artifactSatisfied(coord) {
			res.SkippedInstalled = append(res.SkippedInstalled, m)
			continue
		}
		res.BuildOrder = append(res.BuildOrder, m)
	}
	return res, nil
}

func (w *Workspace) collectInternalEdges() []InternalEdge {
	var edges []InternalEdge
	seen := map[string]struct{}{}
	for i := range w.modules {
		consumer := &w.modules[i]
		for _, dep := range consumer.Dependencies {
			provider := w.matchModule(consumer, dep)
			if provider == nil || provider.ProjectRecID == consumer.ProjectRecID {
				continue
			}
			key := consumer.ProjectRecID + "->" + provider.ProjectRecID + ":" + dep.Key()
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			edges = append(edges, InternalEdge{
				ConsumerRecID: consumer.ProjectRecID,
				ConsumerName:  consumer.ProjectName,
				ProviderRecID: provider.ProjectRecID,
				ProviderName:  provider.ProjectName,
				Dependency:    dep,
			})
		}
	}
	return edges
}

// ScanModules parses pom.xml for each project directory.
func ScanModules(entries []ScanEntry, localRepo string) (*Workspace, *AnalysisResult, error) {
	return ScanModulesWithMavenHome(entries, localRepo, "")
}

// ScanModulesWithMavenHome parses pom.xml with optional MAVEN_HOME for artifact checks.
func ScanModulesWithMavenHome(entries []ScanEntry, localRepo, mavenHome string) (*Workspace, *AnalysisResult, error) {
	modules := make([]Module, 0, len(entries))
	meta := &AnalysisResult{}
	lookup := buildPOMLookup(entries)
	for _, e := range entries {
		pomPath := filepath.Join(e.ProjectDir, "pom.xml")
		pom, err := ParsePOMFile(pomPath)
		if err != nil {
			meta.MissingPOM = append(meta.MissingPOM, e.ProjectName+": "+err.Error())
			continue
		}
		moduleCoords, err := ResolveModuleCoordinates(pomPath, localRepo, lookup, e.ProjectName, "")
		if err != nil {
			meta.MissingPOM = append(meta.MissingPOM, e.ProjectName+": "+err.Error())
			continue
		}
		fallbacks := CollectEffectiveProperties(pomPath, localRepo, lookup, nil)
		deps, err := EffectiveDependencies(pomPath, localRepo, lookup, fallbacks)
		if err != nil {
			deps = make([]Coordinates, 0, len(pom.Dependencies))
			for _, d := range pom.Dependencies {
				d.Version = pom.ResolveVersion(d.Version, fallbacks)
				deps = append(deps, d)
			}
		}
		modules = append(modules, Module{
			ProjectRecID:   e.ProjectRecID,
			ProjectName:    e.ProjectName,
			ProjectVersion: e.ProjectVersion,
			PomPath:        pomPath,
			Coordinates:    moduleCoords,
			Dependencies:   deps,
		})
	}
	return NewWorkspaceWithMavenHome(modules, localRepo, mavenHome), meta, nil
}

// ScanEntry is one workspace project directory to analyze.
type ScanEntry struct {
	ProjectRecID   string
	ProjectName    string
	ProjectVersion string
	ProjectDir     string
}

// DefaultInstallCmd is the command used to install upstream modules into ~/.m2.
const DefaultInstallCmd = "mvn clean install -DskipTests"
