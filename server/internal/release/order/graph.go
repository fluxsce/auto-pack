// Package order 按工作空间内 Maven POM 依赖做拓扑排序，供自动构建使用。
package order

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release/notes"
)

var (
	rePomGroup    = regexp.MustCompile(`(?s)<groupId>\s*([^<]+?)\s*</groupId>`)
	rePomArtifact = regexp.MustCompile(`(?s)<artifactId>\s*([^<]+?)\s*</artifactId>`)
	rePomVersion  = regexp.MustCompile(`(?s)<version>\s*([^<]+?)\s*</version>`)
	rePomDepBlock = regexp.MustCompile(`(?s)<dependency>.*?</dependency>`)
	rePomParent   = regexp.MustCompile(`(?s)<parent>.*?</parent>`)
	rePomDepMgmt  = regexp.MustCompile(`(?s)<dependencyManagement>.*?</dependencyManagement>`)
	rePomPlugin   = regexp.MustCompile(`(?s)<plugin>.*?</plugin>`)
	rePomHeadCut  = regexp.MustCompile(`(?is)<(dependencyManagement|dependencies|build|profiles|modules)\b`)
)

// MavenModule is one workspace Maven project in build order.
type MavenModule struct {
	ProjectRecID  string `json:"project_rec_id"`
	ProjectName   string `json:"project_name"`
	Coordinates   string `json:"coordinates,omitempty"`
	LastJobStatus string `json:"last_job_status,omitempty"`
	IsEnabled     int    `json:"is_enabled"`
}

// MavenEdge is a workspace-internal Maven dependency.
type MavenEdge struct {
	ConsumerRecID string `json:"consumer_rec_id"`
	ConsumerName  string `json:"consumer_name"`
	ProviderRecID string `json:"provider_rec_id"`
	ProviderName  string `json:"provider_name"`
}

// MavenCycle is a strongly connected group that could not be ordered uniquely.
type MavenCycle struct {
	ProjectRecIDs []string `json:"project_rec_ids"`
	ProjectNames  []string `json:"project_names,omitempty"`
}

// MavenAnalysis is the workspace Maven graph used by auto-build.
type MavenAnalysis struct {
	FullBuildOrder []MavenModule  `json:"full_build_order"`
	InternalEdges  []MavenEdge    `json:"internal_edges"`
	CyclicGroups   []MavenCycle   `json:"cyclic_groups,omitempty"`
	UnmatchedDeps  []UnmatchedDep `json:"unmatched_deps,omitempty"`
	Warnings       []string       `json:"warnings,omitempty"`
}

// UnmatchedDep is a pom.xml dependency that is not another workspace project.
// Only SNAPSHOT or same-group coordinates are kept (private modules that would
// surface as "The POM for ... is missing" when built alone).
type UnmatchedDep struct {
	ConsumerRecID string `json:"consumer_rec_id"`
	ConsumerName  string `json:"consumer_name"`
	Coordinates   string `json:"coordinates"`
}

// pomDep is one parsed dependency coordinate used while building the graph.
type pomDep struct {
	ga      string
	version string
}

// mavenNode is one parsed Maven project used while building the workspace graph.
type mavenNode struct {
	prj  project.Project
	ga   string
	deps []pomDep
}

// AnalyzeMavenWorkspace builds a workspace graph: Maven POM plus npm / Node / Vue package.json.
func AnalyzeMavenWorkspace(list []project.Project, workDir func(project.Project) string) MavenAnalysis {
	nodes := make([]mavenNode, 0, len(list))
	unparsed := make([]project.Project, 0)
	byGA := map[string]project.Project{}
	for _, prj := range list {
		dir := ""
		if workDir != nil {
			dir = workDir(prj)
		}
		var ga string
		var deps []pomDep
		switch {
		case notes.IsMavenReleaseProject(prj.ProjectType):
			ga, deps = parsePomGraph(filepath.Join(dir, "pom.xml"))
		case isNodeOrderProject(prj.ProjectType):
			ga, deps = parsePackageGraph(packageJSONPath(dir, prj))
		default:
			continue
		}
		if ga == "" {
			unparsed = append(unparsed, prj)
			continue
		}
		nodes = append(nodes, mavenNode{prj: prj, ga: ga, deps: deps})
		byGA[ga] = prj
	}
	out := MavenAnalysis{}
	incoming := map[string]int{}
	edges := map[string][]string{}
	workspaceGroups := map[string]bool{}
	for _, n := range nodes {
		if g := coordGroup(n.ga); g != "" {
			workspaceGroups[g] = true
		}
	}
	for _, n := range nodes {
		incoming[n.prj.RecID] = incoming[n.prj.RecID]
		for _, dep := range n.deps {
			prov, ok := byGA[dep.ga]
			if ok && prov.RecID != n.prj.RecID {
				out.InternalEdges = append(out.InternalEdges, MavenEdge{
					ConsumerRecID: n.prj.RecID, ConsumerName: n.prj.ProjectName,
					ProviderRecID: prov.RecID, ProviderName: prov.ProjectName,
				})
				edges[prov.RecID] = append(edges[prov.RecID], n.prj.RecID)
				incoming[n.prj.RecID]++
				continue
			}
			if ok || !flagUnmatchedDep(dep, n.ga, workspaceGroups) {
				continue
			}
			out.UnmatchedDeps = append(out.UnmatchedDeps, UnmatchedDep{
				ConsumerRecID: n.prj.RecID,
				ConsumerName:  n.prj.ProjectName,
				Coordinates:   formatPomCoord(dep),
			})
		}
	}
	byID := map[string]mavenNode{}
	for _, n := range nodes {
		byID[n.prj.RecID] = n
	}
	queue := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if incoming[n.prj.RecID] == 0 {
			queue = append(queue, n.prj.RecID)
		}
	}
	sortIDsByName(queue, byID)
	seen := map[string]bool{}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if seen[id] {
			continue
		}
		seen[id] = true
		n := byID[id]
		out.FullBuildOrder = append(out.FullBuildOrder, moduleOf(n.prj, n.ga))
		for _, next := range edges[id] {
			incoming[next]--
			if incoming[next] == 0 {
				queue = append(queue, next)
			}
		}
		// 同层无依赖模块按工程名排出，预览与实际队列稳定一致。
		sortIDsByName(queue, byID)
	}
	var leftover []mavenNode
	for _, n := range nodes {
		if !seen[n.prj.RecID] {
			leftover = append(leftover, n)
		}
	}
	sort.SliceStable(leftover, func(i, j int) bool {
		return leftover[i].prj.ProjectName < leftover[j].prj.ProjectName
	})
	if len(leftover) > 0 {
		ids := make([]string, 0, len(leftover))
		names := make([]string, 0, len(leftover))
		for _, n := range leftover {
			ids = append(ids, n.prj.RecID)
			names = append(names, n.prj.ProjectName)
			out.FullBuildOrder = append(out.FullBuildOrder, moduleOf(n.prj, n.ga))
		}
		out.CyclicGroups = append(out.CyclicGroups, MavenCycle{ProjectRecIDs: ids, ProjectNames: names})
		out.Warnings = append(out.Warnings, "检测到循环依赖，按工程名追加剩余模块")
	}
	sort.SliceStable(unparsed, func(i, j int) bool {
		return unparsed[i].ProjectName < unparsed[j].ProjectName
	})
	for _, prj := range unparsed {
		out.FullBuildOrder = append(out.FullBuildOrder, moduleOf(prj, ""))
		out.Warnings = append(out.Warnings, fmt.Sprintf("工程 %s 未解析到有效 pom.xml / package.json，已追加到队尾", prj.ProjectName))
	}
	return out
}

func moduleOf(prj project.Project, ga string) MavenModule {
	return MavenModule{
		ProjectRecID:  prj.RecID,
		ProjectName:   prj.ProjectName,
		Coordinates:   ga,
		LastJobStatus: prj.LastJobStatus,
		IsEnabled:     prj.IsEnabled,
	}
}

func sortIDsByName(ids []string, byID map[string]mavenNode) {
	sort.SliceStable(ids, func(i, j int) bool {
		return byID[ids[i]].prj.ProjectName < byID[ids[j]].prj.ProjectName
	})
}

// parsePomGraph 读工程根 pom.xml：坐标取项目头（可继承 parent.groupId），
// 依赖只取正式 <dependencies>，忽略 dependencyManagement 与插件依赖。
func parsePomGraph(path string) (ga string, deps []pomDep) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}
	content := string(raw)
	parentBlock := rePomParent.FindString(content)
	parentG := firstCapture(rePomGroup, parentBlock)
	parentV := firstCapture(rePomVersion, parentBlock)

	head := content
	if loc := rePomHeadCut.FindStringIndex(content); loc != nil {
		head = content[:loc[0]]
	}
	head = rePomParent.ReplaceAllString(head, "")
	g := resolveCoordToken(firstCapture(rePomGroup, head), parentG, "")
	if g == "" {
		g = parentG
	}
	a := resolveCoordToken(firstCapture(rePomArtifact, head), parentG, g)
	if g == "" || a == "" {
		return "", nil
	}
	ga = g + ":" + a
	projectV := resolveVersionToken(firstCapture(rePomVersion, head), "", parentV)

	body := rePomParent.ReplaceAllString(content, "")
	body = rePomDepMgmt.ReplaceAllString(body, "")
	body = rePomPlugin.ReplaceAllString(body, "")
	seen := map[string]bool{}
	for _, block := range rePomDepBlock.FindAllString(body, -1) {
		dg := resolveCoordToken(firstCapture(rePomGroup, block), parentG, g)
		da := strings.TrimSpace(firstCapture(rePomArtifact, block))
		if dg == "" || da == "" {
			continue
		}
		coord := dg + ":" + da
		if seen[coord] {
			continue
		}
		seen[coord] = true
		deps = append(deps, pomDep{
			ga:      coord,
			version: resolveVersionToken(firstCapture(rePomVersion, block), projectV, parentV),
		})
	}
	return ga, deps
}

func resolveVersionToken(raw, projectVer, parentVer string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	switch s {
	case "${project.version}", "${pom.version}":
		return projectVer
	case "${project.parent.version}", "${parent.version}", "${pom.parent.version}":
		return parentVer
	default:
		if strings.Contains(s, "${") {
			return ""
		}
		return s
	}
}

func coordGroup(ga string) string {
	g, _, ok := strings.Cut(strings.TrimSpace(ga), ":")
	if !ok {
		return ""
	}
	return g
}

func isSnapshotVersion(ver string) bool {
	return strings.Contains(strings.ToUpper(ver), "SNAPSHOT")
}

func flagUnmatchedDep(dep pomDep, consumerGA string, workspaceGroups map[string]bool) bool {
	if dep.ga == "" || dep.ga == consumerGA {
		return false
	}
	if isSnapshotVersion(dep.version) {
		return true
	}
	g := coordGroup(dep.ga)
	if g == "" {
		return false
	}
	return g == coordGroup(consumerGA) || workspaceGroups[g]
}

func formatPomCoord(dep pomDep) string {
	if dep.version == "" {
		return dep.ga
	}
	return dep.ga + ":" + dep.version
}

func resolveCoordToken(raw, parentGroup, projectGroup string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	switch s {
	case "${project.groupId}", "${pom.groupId}":
		if projectGroup != "" {
			return projectGroup
		}
		return parentGroup
	case "${project.parent.groupId}", "${parent.groupId}", "${pom.parent.groupId}":
		return parentGroup
	default:
		if strings.Contains(s, "${") {
			return ""
		}
		return s
	}
}

func isNodeOrderProject(projectType string) bool {
	switch strings.ToLower(strings.TrimSpace(projectType)) {
	case "npm", "node", "nodejs", "vue", "frontend":
		return true
	default:
		return false
	}
}

func packageJSONPath(dir string, prj project.Project) string {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	extra := strings.TrimSpace(recipe.Workdir)
	if extra != "" {
		p := filepath.Join(dir, filepath.FromSlash(extra), "package.json")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return filepath.Join(dir, "package.json")
}

// parsePackageGraph 读 package.json：坐标是 name，边只连空间内其它包（dependencies / peer / dev）。
func parsePackageGraph(path string) (name string, deps []pomDep) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil
	}
	var doc struct {
		Name             string            `json:"name"`
		Dependencies     map[string]string `json:"dependencies"`
		DevDependencies  map[string]string `json:"devDependencies"`
		PeerDependencies map[string]string `json:"peerDependencies"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", nil
	}
	name = strings.TrimSpace(doc.Name)
	if name == "" {
		return "", nil
	}
	seen := map[string]bool{}
	add := func(m map[string]string) {
		for key := range m {
			key = strings.TrimSpace(key)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			deps = append(deps, pomDep{ga: key})
		}
	}
	add(doc.Dependencies)
	add(doc.PeerDependencies)
	add(doc.DevDependencies)
	return name, deps
}

func firstCapture(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}
