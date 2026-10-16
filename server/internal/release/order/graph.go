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
	rePomGroup      = regexp.MustCompile(`(?s)<groupId>\s*([^<]+?)\s*</groupId>`)
	rePomArtifact   = regexp.MustCompile(`(?s)<artifactId>\s*([^<]+?)\s*</artifactId>`)
	rePomVersion    = regexp.MustCompile(`(?s)<version>\s*([^<]+?)\s*</version>`)
	rePomScope      = regexp.MustCompile(`(?s)<scope>\s*([^<]+?)\s*</scope>`)
	rePomType       = regexp.MustCompile(`(?s)<type>\s*([^<]+?)\s*</type>`)
	rePomDepBlock   = regexp.MustCompile(`(?s)<dependency>.*?</dependency>`)
	rePomParent     = regexp.MustCompile(`(?s)<parent>.*?</parent>`)
	rePomDepMgmt    = regexp.MustCompile(`(?s)<dependencyManagement>.*?</dependencyManagement>`)
	rePomPlugin     = regexp.MustCompile(`(?s)<plugin>.*?</plugin>`)
	rePomProperties = regexp.MustCompile(`(?s)<properties>(.*?)</properties>`)
	rePomPropEntry  = regexp.MustCompile(`(?s)<([A-Za-z_][A-Za-z0-9_.-]*)>\s*([^<]*?)\s*</([A-Za-z_][A-Za-z0-9_.-]*)>`)
	rePomHeadCut    = regexp.MustCompile(`(?is)<(dependencyManagement|dependencies|build|profiles|modules)\b`)
	rePomModules    = regexp.MustCompile(`(?s)<modules>.*?</modules>`)
	rePomModule     = regexp.MustCompile(`(?s)<module>\s*([^<]+?)\s*</module>`)
	rePomPropToken  = regexp.MustCompile(`\$\{([A-Za-z_][A-Za-z0-9_.-]*)\}`)
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
	prj      project.Project
	ga       string
	extraGAs []string
	deps     []pomDep
}

// AnalyzeMavenWorkspace builds a workspace graph: Maven POM plus npm / Node / Vue package.json.
func AnalyzeMavenWorkspace(list []project.Project, workDir func(project.Project) string) MavenAnalysis {
	out := MavenAnalysis{}
	nodes := make([]mavenNode, 0, len(list))
	unparsed := make([]project.Project, 0)
	byGA := map[string]project.Project{}
	for _, prj := range list {
		dir := ""
		if workDir != nil {
			dir = workDir(prj)
		}
		var ga string
		var extra []string
		var deps []pomDep
		switch {
		case notes.IsMavenReleaseProject(prj.ProjectType):
			ga, extra, deps = parsePomReactor(pomXMLPath(dir, prj))
		case isNodeOrderProject(prj.ProjectType):
			ga, deps = parsePackageGraph(packageJSONPath(dir, prj))
		default:
			continue
		}
		if ga == "" {
			unparsed = append(unparsed, prj)
			continue
		}
		nodes = append(nodes, mavenNode{prj: prj, ga: ga, extraGAs: extra, deps: deps})
		if prev, ok := byGA[ga]; ok && prev.RecID != prj.RecID {
			out.Warnings = append(out.Warnings, fmt.Sprintf(
				"坐标 %s 同时对应工程 %s 与 %s，排序以先登记的为准",
				ga, prev.ProjectName, prj.ProjectName,
			))
			continue
		}
		byGA[ga] = prj
	}
	// Nested Spring Boot / reactor modules share the workspace project.
	// A dedicated project whose root POM matches the GA still wins.
	for _, n := range nodes {
		for _, extra := range n.extraGAs {
			if extra == "" || extra == n.ga {
				continue
			}
			if prev, ok := byGA[extra]; ok {
				if prev.RecID != n.prj.RecID {
					out.Warnings = append(out.Warnings, fmt.Sprintf(
						"坐标 %s 同时对应工程 %s 与嵌套模块（%s），排序以已登记工程为准",
						extra, prev.ProjectName, n.prj.ProjectName,
					))
				}
				continue
			}
			byGA[extra] = n.prj
		}
	}
	incoming := map[string]int{}
	edges := map[string][]string{}
	workspaceGroups := map[string]bool{}
	seenEdge := map[string]bool{}
	seenDisabledWarn := map[string]bool{}
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
				ek := n.prj.RecID + ">" + prov.RecID
				if !seenEdge[ek] {
					seenEdge[ek] = true
					out.InternalEdges = append(out.InternalEdges, MavenEdge{
						ConsumerRecID: n.prj.RecID, ConsumerName: n.prj.ProjectName,
						ProviderRecID: prov.RecID, ProviderName: prov.ProjectName,
					})
					edges[prov.RecID] = append(edges[prov.RecID], n.prj.RecID)
					incoming[n.prj.RecID]++
				}
				if prov.IsEnabled == 0 && !seenDisabledWarn[ek] {
					seenDisabledWarn[ek] = true
					out.Warnings = append(out.Warnings, fmt.Sprintf(
						"工程 %s 依赖已停用的 %s，自动构建将跳过该下游",
						n.prj.ProjectName, prov.ProjectName,
					))
				}
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

const pomReactorDepth = 6

// parsePomReactor 读工程根 pom.xml，并沿 <modules> 展开 Spring Boot / 多模块反应器。
// 坐标取项目头（可继承 parent.groupId）；依赖合并各模块正式 <dependencies>。
func parsePomReactor(path string) (ga string, extra []string, deps []pomDep) {
	seen := map[string]bool{}
	ga, extra, deps = collectPomReactor(path, seen, 0)
	return ga, extra, uniquePomDeps(deps)
}

func collectPomReactor(path string, seen map[string]bool, depth int) (ga string, extra []string, deps []pomDep) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = filepath.Clean(path)
	}
	if seen[abs] || depth > pomReactorDepth {
		return "", nil, nil
	}
	seen[abs] = true
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", nil, nil
	}
	content := string(raw)
	ga, deps = parsePomCoords(content)
	if ga != "" {
		extra = append(extra, ga)
	}
	dir := filepath.Dir(path)
	for _, rel := range pomModuleRels(content) {
		child := filepath.Join(dir, filepath.FromSlash(rel), "pom.xml")
		if !pomChildInRoot(dir, child) {
			continue
		}
		cga, cextra, cdeps := collectPomReactor(child, seen, depth+1)
		if cga != "" && cga != ga {
			extra = append(extra, cga)
		}
		extra = append(extra, cextra...)
		deps = append(deps, cdeps...)
	}
	return ga, uniqueStrings(extra), deps
}

func pomXMLPath(dir string, prj project.Project) string {
	recipe := project.ParseRecipe(prj.RecipeJSON)
	extra := strings.TrimSpace(recipe.Workdir)
	if extra != "" {
		p := filepath.Join(dir, filepath.FromSlash(extra), "pom.xml")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return filepath.Join(dir, "pom.xml")
}

func pomModuleRels(content string) []string {
	block := rePomModules.FindString(content)
	if block == "" {
		return nil
	}
	var out []string
	for _, m := range rePomModule.FindAllStringSubmatch(block, -1) {
		if len(m) < 2 {
			continue
		}
		rel := sanitizePomModuleRel(m[1])
		if rel != "" {
			out = append(out, rel)
		}
	}
	return out
}

func sanitizePomModuleRel(raw string) string {
	s := strings.Trim(strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/")), "/")
	if s == "" || strings.HasPrefix(s, "/") || strings.Contains(s, ":") {
		return ""
	}
	for _, part := range strings.Split(s, "/") {
		if part == "" || part == ".." {
			return ""
		}
	}
	return s
}

func pomChildInRoot(root, child string) bool {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return false
	}
	absChild, err := filepath.Abs(child)
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(absRoot, absChild)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func uniquePomDeps(deps []pomDep) []pomDep {
	seen := map[string]bool{}
	out := make([]pomDep, 0, len(deps))
	for _, dep := range deps {
		if dep.ga == "" || seen[dep.ga] {
			continue
		}
		seen[dep.ga] = true
		out = append(out, dep)
	}
	return out
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// parsePomCoords 读单个 pom.xml：坐标取项目头；边含 <parent>、import BOM、正式 <dependencies>。
// 本文件 <properties> 做字面量替换（不递归读父 POM）。
func parsePomCoords(content string) (ga string, deps []pomDep) {
	props := parsePomProperties(content)
	parentBlock := rePomParent.FindString(content)
	parentG := resolvePropertyRefs(firstCapture(rePomGroup, parentBlock), props)
	parentA := resolvePropertyRefs(firstCapture(rePomArtifact, parentBlock), props)
	parentV := resolvePropertyRefs(firstCapture(rePomVersion, parentBlock), props)

	head := content
	if loc := rePomHeadCut.FindStringIndex(content); loc != nil {
		head = content[:loc[0]]
	}
	head = rePomParent.ReplaceAllString(head, "")
	g := resolveCoordToken(firstCapture(rePomGroup, head), parentG, "")
	if g == "" {
		g = parentG
	}
	g = resolvePropertyRefs(g, props)
	a := resolveCoordToken(firstCapture(rePomArtifact, head), parentG, g)
	a = resolvePropertyRefs(a, props)
	if g == "" || a == "" {
		return "", nil
	}
	ga = g + ":" + a
	projectV := resolveVersionToken(firstCapture(rePomVersion, head), "", parentV, props)

	seen := map[string]bool{}
	addDep := func(dg, da, ver string) {
		dg = strings.TrimSpace(dg)
		da = strings.TrimSpace(da)
		if dg == "" || da == "" {
			return
		}
		coord := dg + ":" + da
		if coord == ga || seen[coord] {
			return
		}
		seen[coord] = true
		deps = append(deps, pomDep{ga: coord, version: ver})
	}

	// parent 本身也要 install；同反应器子模块 parent 指向本工程时 coord==ga 被跳过。
	if parentG != "" && parentA != "" {
		addDep(parentG, parentA, parentV)
	}
	// dependencyManagement 里 scope=import + type=pom 的 BOM 需先构建。
	for _, block := range importBOMBlocks(content) {
		dg := resolvePropertyRefs(resolveCoordToken(firstCapture(rePomGroup, block), parentG, g), props)
		da := resolvePropertyRefs(strings.TrimSpace(firstCapture(rePomArtifact, block)), props)
		ver := resolveVersionToken(firstCapture(rePomVersion, block), projectV, parentV, props)
		addDep(dg, da, ver)
	}

	body := rePomParent.ReplaceAllString(content, "")
	body = rePomDepMgmt.ReplaceAllString(body, "")
	body = rePomPlugin.ReplaceAllString(body, "")
	for _, block := range rePomDepBlock.FindAllString(body, -1) {
		dg := resolvePropertyRefs(resolveCoordToken(firstCapture(rePomGroup, block), parentG, g), props)
		da := resolvePropertyRefs(strings.TrimSpace(firstCapture(rePomArtifact, block)), props)
		ver := resolveVersionToken(firstCapture(rePomVersion, block), projectV, parentV, props)
		addDep(dg, da, ver)
	}
	return ga, deps
}

func importBOMBlocks(content string) []string {
	var out []string
	for _, dm := range rePomDepMgmt.FindAllString(content, -1) {
		for _, block := range rePomDepBlock.FindAllString(dm, -1) {
			scope := strings.ToLower(strings.TrimSpace(firstCapture(rePomScope, block)))
			typ := strings.ToLower(strings.TrimSpace(firstCapture(rePomType, block)))
			if scope == "import" && typ == "pom" {
				out = append(out, block)
			}
		}
	}
	return out
}

// parsePomProperties 只读本 POM 的 <properties> 字面量，不做父 POM 继承。
func parsePomProperties(content string) map[string]string {
	body := rePomParent.ReplaceAllString(content, "")
	block := rePomProperties.FindStringSubmatch(body)
	if len(block) < 2 {
		return nil
	}
	props := map[string]string{}
	for _, m := range rePomPropEntry.FindAllStringSubmatch(block[1], -1) {
		if len(m) < 4 || m[1] != m[3] {
			continue
		}
		key := strings.TrimSpace(m[1])
		if key == "" || strings.EqualFold(key, "properties") {
			continue
		}
		props[key] = strings.TrimSpace(m[2])
	}
	return props
}

// resolvePropertyRefs 用本文件 properties 替换 ${key}；仍含 ${} 则视为未解析。
func resolvePropertyRefs(raw string, props map[string]string) string {
	s := strings.TrimSpace(raw)
	if s == "" || !strings.Contains(s, "${") || len(props) == 0 {
		if strings.Contains(s, "${") {
			return ""
		}
		return s
	}
	out := s
	for pass := 0; pass < 4 && strings.Contains(out, "${"); pass++ {
		changed := false
		for _, m := range rePomPropToken.FindAllStringSubmatch(out, -1) {
			if len(m) < 2 {
				continue
			}
			key := m[1]
			val, ok := props[key]
			if !ok || strings.Contains(val, "${") {
				continue
			}
			next := strings.ReplaceAll(out, m[0], val)
			if next != out {
				out = next
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	if strings.Contains(out, "${") {
		return ""
	}
	return out
}

func resolveVersionToken(raw, projectVer, parentVer string, props map[string]string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	switch s {
	case "${project.version}", "${pom.version}":
		if projectVer != "" {
			return projectVer
		}
		return resolvePropertyRefs(s, props)
	case "${project.parent.version}", "${parent.version}", "${pom.parent.version}":
		return parentVer
	default:
		if strings.Contains(s, "${") {
			return resolvePropertyRefs(s, props)
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
		// 其它 ${...} 留给 resolvePropertyRefs 用本 POM properties 展开。
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
