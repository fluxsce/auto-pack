package maven

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/server/internal/host/bash"
)

const (
	// TreeCommand 固定 dependency-plugin 3.7.1，写出 JSON 有效树。
	TreeCommand = "mvn -B -DskipTests org.apache.maven.plugins:maven-dependency-plugin:3.7.1:tree"
	treeOutput  = "target/auto-pack-deps.json"
	treeTimeout = 2 * time.Minute
)

// Node 是 dependency:tree 里的一行。
type Node struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Extra   string `json:"extra,omitempty"`
	Source  string `json:"source,omitempty"`
	Depth   int    `json:"depth,omitempty"`
	Note    string `json:"note,omitempty"`
}

type jsonNode struct {
	GroupID    string     `json:"groupId"`
	ArtifactID string     `json:"artifactId"`
	Version    string     `json:"version"`
	Type       string     `json:"type"`
	Scope      string     `json:"scope"`
	Classifier string     `json:"classifier"`
	Optional   any        `json:"optional"`
	Children   []jsonNode `json:"children"`
}

// Tree 跑 Maven 有效树：优先读插件 JSON，失败再回落文本。
func Tree(ctx context.Context, dir, command string, env []string) ([]Node, error) {
	if strings.TrimSpace(command) == "" {
		command = TreeCommand
	}
	if !strings.Contains(command, "-DoutputType=") {
		command = strings.TrimSpace(command) + " -Dverbose -DoutputType=json -DoutputFile=" + treeOutput
	}
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, treeTimeout)
		defer cancel()
	}
	var buf bytes.Buffer
	err := bash.Run(ctx, dir, command, env, &buf)
	out := buf.String()
	nodes := collectJSON(dir)
	if len(nodes) == 0 {
		nodes = ParseTree(out)
	} else {
		annotateConflicts(nodes)
	}
	if len(nodes) > 0 {
		return nodes, nil
	}
	if err != nil {
		return nil, fmt.Errorf("maven dependency:tree: %w\n%s", err, tailLines(out, 20))
	}
	return nodes, nil
}

func collectJSON(root string) []Node {
	var nodes []Node
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if path != root && (strings.HasPrefix(name, ".") || name == "node_modules" || name == "dist" || name == "vendor") {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "auto-pack-deps.json" {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		nodes = append(nodes, ParseJSON(raw)...)
		return nil
	})
	return nodes
}

// ParseJSON 解析 dependency-plugin 的 tree JSON（单模块一个根对象）。
func ParseJSON(raw []byte) []Node {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return nil
	}
	var root jsonNode
	if err := json.Unmarshal(raw, &root); err != nil || root.ArtifactID == "" {
		var many []jsonNode
		if err := json.Unmarshal(raw, &many); err != nil {
			return nil
		}
		var out []Node
		for _, item := range many {
			out = append(out, flattenJSON(item, 0, "")...)
		}
		return out
	}
	return flattenJSON(root, 0, "")
}

func flattenJSON(n jsonNode, depth int, source string) []Node {
	if strings.TrimSpace(n.ArtifactID) == "" {
		return nil
	}
	name := n.GroupID + ":" + n.ArtifactID
	if c := strings.TrimSpace(n.Classifier); c != "" {
		name += ":" + c
	}
	extra := strings.TrimSpace(n.Scope)
	if depth == 0 {
		extra = "module"
		source = name
		if n.Version != "" {
			source += ":" + n.Version
		}
	}
	if extra == "" && depth > 0 {
		extra = "compile"
	}
	if jsonOptional(n.Optional) && extra != "module" {
		extra = "optional"
	}
	out := []Node{{
		Name:    name,
		Version: strings.TrimSpace(n.Version),
		Extra:   extra,
		Source:  source,
		Depth:   depth,
	}}
	for _, child := range n.Children {
		out = append(out, flattenJSON(child, depth+1, source)...)
	}
	return out
}

func jsonOptional(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(strings.TrimSpace(t), "true")
	default:
		return false
	}
}

func annotateConflicts(nodes []Node) {
	type key struct{ source, name string }
	type win struct {
		version string
		depth   int
	}
	winner := map[key]win{}
	for _, n := range nodes {
		if n.Depth == 0 {
			continue
		}
		k := key{n.Source, n.Name}
		cur, ok := winner[k]
		if !ok || n.Depth < cur.depth {
			winner[k] = win{version: n.Version, depth: n.Depth}
		}
	}
	for i, n := range nodes {
		if n.Depth == 0 {
			continue
		}
		w := winner[key{n.Source, n.Name}]
		if w.version == "" || w.version == n.Version {
			continue
		}
		nodes[i].Extra = "conflict"
		nodes[i].Note = "omitted for conflict with " + w.version
	}
}

// ParseTree 解析 `mvn dependency:tree` 的文本输出（JSON 不可用时的回落）。
func ParseTree(raw string) []Node {
	var nodes []Node
	module := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		line = strings.TrimPrefix(line, "[INFO] ")
		line = strings.TrimPrefix(line, "[WARNING] ")
		if line == "" || skipTreeNoise(line) {
			continue
		}
		depth, rest := splitTreePrefix(line)
		note := ""
		if i := strings.Index(rest, " ("); i >= 0 {
			note = strings.TrimSuffix(strings.TrimSpace(rest[i+2:]), ")")
			rest = strings.TrimSpace(rest[:i])
		}
		coord, ok := parseCoord(rest)
		if !ok {
			continue
		}
		if depth == 0 {
			module = coord.name
			if coord.version != "" {
				module += ":" + coord.version
			}
			coord.extra = "module"
		}
		if coord.extra == "" {
			coord.extra = "compile"
		}
		if strings.Contains(strings.ToLower(note), "conflict") {
			coord.extra = "conflict"
		}
		nodes = append(nodes, Node{
			Name:    coord.name,
			Version: coord.version,
			Extra:   coord.extra,
			Source:  module,
			Depth:   depth,
			Note:    note,
		})
	}
	return nodes
}

type parsedCoord struct {
	name    string
	version string
	extra   string
}

func parseCoord(raw string) (parsedCoord, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return parsedCoord{}, false
	}
	if i := strings.IndexAny(raw, " \t"); i >= 0 {
		raw = raw[:i]
	}
	parts := strings.Split(raw, ":")
	if len(parts) < 4 {
		return parsedCoord{}, false
	}
	for _, p := range parts {
		if p == "" {
			return parsedCoord{}, false
		}
	}
	group, artifact := parts[0], parts[1]
	if strings.ContainsAny(group, " /\\") || strings.ContainsAny(artifact, " /\\") {
		return parsedCoord{}, false
	}
	out := parsedCoord{name: group + ":" + artifact}
	switch len(parts) {
	case 4:
		out.version = parts[3]
	case 5:
		out.version = parts[3]
		out.extra = parts[4]
	default:
		out.version = parts[len(parts)-2]
		out.extra = parts[len(parts)-1]
		if classifier := parts[3]; classifier != "" && len(parts) >= 6 {
			out.name += ":" + classifier
		}
	}
	return out, out.version != ""
}

func splitTreePrefix(line string) (depth int, rest string) {
	rest = line
	for rest != "" {
		switch {
		case strings.HasPrefix(rest, "|  "):
			depth++
			rest = rest[3:]
		case strings.HasPrefix(rest, "| "):
			depth++
			rest = rest[2:]
		case strings.HasPrefix(rest, "+- "), strings.HasPrefix(rest, "\\- "):
			return depth + 1, strings.TrimSpace(rest[3:])
		case strings.HasPrefix(rest, "+-"), strings.HasPrefix(rest, "\\-"):
			return depth + 1, strings.TrimSpace(rest[2:])
		default:
			return depth, strings.TrimSpace(rest)
		}
	}
	return depth, rest
}

func skipTreeNoise(line string) bool {
	switch {
	case strings.Contains(line, "-----") || strings.HasPrefix(line, "---"):
		return true
	case strings.HasPrefix(line, "Scanning "), strings.HasPrefix(line, "Building "), strings.HasPrefix(line, "Finished "):
		return true
	case strings.HasPrefix(line, "Reactor "), strings.HasPrefix(line, "Downloading"), strings.HasPrefix(line, "Downloaded"):
		return true
	case strings.HasPrefix(line, "BUILD "), strings.HasPrefix(line, "Total time"), strings.HasPrefix(line, "Finished at"):
		return true
	default:
		return false
	}
}

func tailLines(raw string, n int) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || n <= 0 {
		return raw
	}
	lines := strings.Split(raw, "\n")
	if len(lines) <= n {
		return raw
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
