package project

import (
	"errors"
	"strings"
)

var (
	// ErrCommandPatch is returned when a batch build rewrite has nothing to apply.
	ErrCommandPatch = errors.New("command find/replace, goal, set_command, or build fields required")
	// ErrCommandFamily is returned when family is not maven, node, or empty (all).
	ErrCommandFamily = errors.New("family must be maven, node, or empty")
	// ErrCommandGoal is returned when goal is not a Maven packaging phase.
	ErrCommandGoal = errors.New("goal must be package, install, or deploy")
)

var mavenPackGoals = map[string]bool{
	"package": true,
	"install": true,
	"deploy":  true,
}

// CommandPatch rewrites the same build fields as the project editor (command / workdir / output / artifacts / retention).
type CommandPatch struct {
	Family           string  `json:"family,omitempty"`
	Goal             string  `json:"goal,omitempty"`
	Find             string  `json:"find,omitempty"`
	Replace          string  `json:"replace,omitempty"`
	SetCommand       string  `json:"set_command,omitempty"`
	Workdir          *string `json:"workdir,omitempty"`
	OutputDir        *string `json:"output_dir,omitempty"`
	ArtifactPattern  *string `json:"artifact_pattern,omitempty"`
	LogRetentionDays *int    `json:"log_retention_days,omitempty"`
}

// CommandUpdate is the result of a batch command rewrite.
type CommandUpdate struct {
	Updated int    `json:"updated"`
	Skipped int    `json:"skipped"`
	Family  string `json:"family,omitempty"`
}

// CheckCommandPatch accepts a batch build rewrite.
func CheckCommandPatch(p CommandPatch) error {
	family := strings.ToLower(strings.TrimSpace(p.Family))
	if family != "" && family != "maven" && family != "node" && family != "all" {
		return ErrCommandFamily
	}
	if goal := normalizePackGoal(p.Goal); goal != "" && !mavenPackGoals[goal] {
		return ErrCommandGoal
	}
	if hasCommandChange(p) || hasBuildMeta(p) {
		return nil
	}
	return ErrCommandPatch
}

func hasCommandChange(p CommandPatch) bool {
	if normalizePackGoal(p.Goal) != "" {
		return true
	}
	if strings.TrimSpace(p.SetCommand) != "" {
		return true
	}
	find := strings.TrimSpace(p.Find)
	repl := strings.TrimSpace(p.Replace)
	return find != "" && repl != "" && find != repl
}

func hasBuildMeta(p CommandPatch) bool {
	return p.Workdir != nil || p.OutputDir != nil || p.ArtifactPattern != nil || p.LogRetentionDays != nil
}

func normalizePackGoal(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// MatchCommandFamily reports whether a project type belongs to the patch family.
func MatchCommandFamily(projectType, family string) bool {
	switch strings.ToLower(strings.TrimSpace(family)) {
	case "", "all":
		return true
	case "maven", "java":
		return isMavenType(projectType)
	case "node":
		return isNodeType(projectType)
	default:
		return false
	}
}

func isMavenType(projectType string) bool {
	switch strings.ToLower(strings.TrimSpace(projectType)) {
	case "maven", "java":
		return true
	default:
		return false
	}
}

func isNodeType(projectType string) bool {
	switch strings.ToLower(strings.TrimSpace(projectType)) {
	case "npm", "node", "nodejs", "vue", "frontend":
		return true
	default:
		return false
	}
}

// PatchRecipe applies the editor build fields. ok is false when nothing changes.
func PatchRecipe(r Recipe, p CommandPatch) (Recipe, bool) {
	changed := false
	if next, ok := PatchCommand(r.Command, p); ok {
		r.Command = next
		changed = true
	}
	if p.Workdir != nil && r.Workdir != *p.Workdir {
		r.Workdir = *p.Workdir
		changed = true
	}
	if p.OutputDir != nil && r.OutputDir != *p.OutputDir {
		r.OutputDir = *p.OutputDir
		changed = true
	}
	if p.ArtifactPattern != nil && r.ArtifactPattern != *p.ArtifactPattern {
		r.ArtifactPattern = *p.ArtifactPattern
		changed = true
	}
	if p.LogRetentionDays != nil && r.LogRetentionDays != *p.LogRetentionDays {
		r.LogRetentionDays = *p.LogRetentionDays
		changed = true
	}
	return r, changed
}

// PatchCommand applies a Maven packaging goal rewrite, token replace, or full-command set.
func PatchCommand(command string, p CommandPatch) (string, bool) {
	if goal := normalizePackGoal(p.Goal); mavenPackGoals[goal] {
		return rewriteMavenGoal(command, goal)
	}
	cmd := strings.TrimSpace(command)
	if set := strings.TrimSpace(p.SetCommand); set != "" {
		if find := strings.TrimSpace(p.Find); find != "" && !commandHasToken(cmd, find) {
			return command, false
		}
		if cmd == set {
			return command, false
		}
		return set, true
	}
	find := strings.TrimSpace(p.Find)
	repl := strings.TrimSpace(p.Replace)
	if find == "" || repl == "" || find == repl {
		return command, false
	}
	return replaceCommandToken(command, find, repl)
}

// rewriteMavenGoal 只替换命令里最后一个 package/install/deploy，保留 clean、-B、-DskipTests。
func rewriteMavenGoal(command, goal string) (string, bool) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return command, false
	}
	last := -1
	for i, part := range parts {
		if mavenPackGoals[part] {
			last = i
		}
	}
	if last < 0 || parts[last] == goal {
		return command, false
	}
	parts[last] = goal
	return strings.Join(parts, " "), true
}

func commandHasToken(command, token string) bool {
	for _, part := range strings.Fields(command) {
		if part == token {
			return true
		}
	}
	return false
}

func replaceCommandToken(command, find, repl string) (string, bool) {
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return command, false
	}
	changed := false
	for i, part := range parts {
		if part == find {
			parts[i] = repl
			changed = true
		}
	}
	if !changed {
		return command, false
	}
	return strings.Join(parts, " "), true
}
