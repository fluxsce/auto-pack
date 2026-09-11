package workspace

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"auto-pack/server/internal/host/maven"
	"auto-pack/server/internal/host/utils"
	"auto-pack/server/internal/iam"
)

var depManifests = []string{"pom.xml", "package.json", "go.mod"}

const depsCacheName = "auto-pack-deps-cache.json"

// DepRow 是依赖分析里的一行。
type DepRow struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Extra   string `json:"extra,omitempty"`
	Source  string `json:"source,omitempty"`
	Depth   int    `json:"depth,omitempty"`
	Note    string `json:"note,omitempty"`
}

// Deps 是工程检出内的依赖分析结果。
type Deps struct {
	Exists  bool     `json:"exists"`
	Cached  bool     `json:"cached,omitempty"`
	Sources []string `json:"sources,omitempty"`
	Direct  int      `json:"direct,omitempty"`
	Rows    []DepRow `json:"rows"`
}

// DepsQuery 是跑依赖分析时用的工具链（绑定的 JDK / Maven 命令与环境）。
type DepsQuery struct {
	ExtraEnv []string
	Command  string
	Workdir  string
	Refresh  bool
}

type depsCache struct {
	Stamp   string   `json:"stamp"`
	Command string   `json:"command"`
	Sources []string `json:"sources"`
	Rows    []DepRow `json:"rows"`
}

// ListDeps 扫描检出清单。Maven 走插件 JSON 有效树，POM 未变时读缓存。
func (s *Service) ListDeps(ctx context.Context, actor iam.Actor, workspaceID, workRel, absLocal string, q DepsQuery) (Deps, error) {
	base, _, err := s.checkoutPath(ctx, actor, workspaceID, workRel, "", absLocal)
	if err != nil {
		return Deps{}, err
	}
	exists, _, err := utils.ListDir(base, "", 1)
	if err != nil {
		return Deps{}, err
	}
	out := Deps{Exists: exists, Rows: []DepRow{}}
	if !exists {
		return out, nil
	}
	paths, err := utils.FindRel(base, depManifests, 80)
	if err != nil {
		return Deps{}, err
	}
	var poms []string
	for _, rel := range paths {
		switch strings.ToLower(path.Base(rel)) {
		case "pom.xml":
			poms = append(poms, rel)
		case "package.json":
			text, _, err := utils.ReadText(base, rel, maxFileBytes)
			if err != nil {
				continue
			}
			out.Sources = append(out.Sources, rel)
			out.Rows = append(out.Rows, parsePackageJSON(rel, text)...)
		case "go.mod":
			text, _, err := utils.ReadText(base, rel, maxFileBytes)
			if err != nil {
				continue
			}
			out.Sources = append(out.Sources, rel)
			out.Rows = append(out.Rows, parseGoMod(rel, text)...)
		}
	}
	if len(poms) == 0 {
		out.Direct = countDirect(out.Rows)
		return out, nil
	}
	dir, source, ok := mavenRoot(base, q.Workdir, poms)
	if !ok {
		return out, nil
	}
	stamp := pomStamp(base, poms)
	cachePath := filepath.Join(dir, "target", depsCacheName)
	if !q.Refresh {
		if cached, ok := readDepsCache(cachePath, stamp, q.Command); ok {
			out.Cached = true
			out.Sources = append([]string{source}, out.Sources...)
			out.Rows = append(cached.Rows, out.Rows...)
			out.Direct = countDirect(out.Rows)
			return out, nil
		}
	}
	nodes, err := maven.Tree(ctx, dir, q.Command, q.ExtraEnv)
	if err != nil {
		return Deps{}, err
	}
	mavenRows := make([]DepRow, 0, len(nodes))
	for _, n := range nodes {
		mavenRows = append(mavenRows, DepRow(n))
	}
	writeDepsCache(cachePath, depsCache{Stamp: stamp, Command: q.Command, Sources: []string{source}, Rows: mavenRows})
	out.Sources = append([]string{source}, out.Sources...)
	out.Rows = append(mavenRows, out.Rows...)
	out.Direct = countDirect(out.Rows)
	return out, nil
}

func countDirect(rows []DepRow) int {
	n := 0
	for _, row := range rows {
		if row.Depth == 1 && row.Extra != "conflict" {
			n++
		}
	}
	return n
}

func pomStamp(base string, poms []string) string {
	h := sha256.New()
	for _, rel := range poms {
		st, err := os.Stat(utils.JoinUnder(base, rel))
		if err != nil {
			fmt.Fprintf(h, "%s\tmissing\n", rel)
			continue
		}
		fmt.Fprintf(h, "%s\t%d\t%d\n", rel, st.Size(), st.ModTime().UnixNano())
	}
	return hex.EncodeToString(h.Sum(nil))
}

func readDepsCache(path, stamp, command string) (depsCache, bool) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return depsCache{}, false
	}
	var c depsCache
	if err := json.Unmarshal(raw, &c); err != nil {
		return depsCache{}, false
	}
	if c.Stamp != stamp || c.Command != command || len(c.Rows) == 0 {
		return depsCache{}, false
	}
	return c, true
}

func writeDepsCache(path string, c depsCache) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, raw, 0o644)
}

func mavenRoot(base, workdir string, poms []string) (abs, source string, ok bool) {
	if rel, err := utils.CleanRel(workdir); err == nil {
		candidate := filepath.Join(utils.JoinUnder(base, rel), "pom.xml")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			src := "pom.xml"
			if rel != "" {
				src = rel + "/pom.xml"
			}
			return filepath.Dir(candidate), src, true
		}
	}
	if len(poms) == 0 {
		return "", "", false
	}
	sort.Slice(poms, func(i, j int) bool {
		if ci, cj := strings.Count(poms[i], "/"), strings.Count(poms[j], "/"); ci != cj {
			return ci < cj
		}
		return poms[i] < poms[j]
	})
	rel := poms[0]
	return filepath.Dir(utils.JoinUnder(base, rel)), rel, true
}

func parsePackageJSON(source, raw string) []DepRow {
	var jsonDoc struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(raw), &jsonDoc); err != nil {
		return nil
	}
	var out []DepRow
	for name, ver := range jsonDoc.Dependencies {
		out = append(out, DepRow{Name: name, Version: ver, Extra: "prod", Source: source, Depth: 1})
	}
	for name, ver := range jsonDoc.DevDependencies {
		out = append(out, DepRow{Name: name, Version: ver, Extra: "dev", Source: source, Depth: 1})
	}
	return out
}

func parseGoMod(source, raw string) []DepRow {
	var out []DepRow
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) < 2 || strings.HasPrefix(fields[0], "//") || fields[0] == "module" || fields[0] == "go" || fields[0] == "exclude" {
			continue
		}
		name, ver := fields[0], fields[1]
		if name == "require" && len(fields) >= 3 {
			name, ver = fields[1], fields[2]
		}
		if !strings.HasPrefix(ver, "v") {
			continue
		}
		out = append(out, DepRow{Name: name, Version: ver, Source: source, Depth: 1})
	}
	return out
}
