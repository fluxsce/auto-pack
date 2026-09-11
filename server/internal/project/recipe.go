package project

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"auto-pack/server/internal/host/utils"
	"auto-pack/server/internal/slug"

	"github.com/robfig/cron/v3"
)

var (
	// ErrRepoURL is returned when scm_mode=git but repo_url is missing or not a git URL.
	ErrRepoURL = errors.New("git repo_url required")
	// ErrRepoBranch is returned when repo_branch is empty or not a git ref name.
	ErrRepoBranch = errors.New("invalid repo_branch")
	// ErrCron is returned when cron_expr is not a 5-field or descriptor expression.
	ErrCron = errors.New("invalid cron_expr")
	// ErrNoArtifacts is returned when the output directory has no files matching the pattern.
	ErrNoArtifacts = errors.New("no artifacts matched")
	// ErrPostKind is returned when a batch post action is not email or shell.
	ErrPostKind = errors.New("unsupported post action kind")
	// ErrPostEnv is returned when an email action has no SMTP env.
	ErrPostEnv = errors.New("post action env required")
	// ErrPostShell is returned when a shell action has no command.
	ErrPostShell = errors.New("post action command required")

	cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
)

// Recipe is the build recipe stored in recipe_json.
type Recipe struct {
	Command          string       `json:"command"`
	Workdir          string       `json:"workdir,omitempty"`
	Artifacts        []string     `json:"artifacts,omitempty"`
	AutoSync         *bool        `json:"auto_sync,omitempty"`
	SCMMode          string       `json:"scm_mode,omitempty"`
	LocalSourcePath  string       `json:"local_source_path,omitempty"`
	OutputDir        string       `json:"output_dir,omitempty"`
	ArtifactPattern  string       `json:"artifact_pattern,omitempty"`
	RemotePath       string       `json:"remote_path,omitempty"`
	GitEnvRecID      string       `json:"git_env_rec_id,omitempty"`
	JDKEnvRecID      string       `json:"jdk_env_rec_id,omitempty"`
	MavenEnvRecID    string       `json:"maven_env_rec_id,omitempty"`
	NodeEnvRecID     string       `json:"node_env_rec_id,omitempty"`
	FTPEnvRecID      string       `json:"ftp_env_rec_id,omitempty"`
	LogRetentionDays int          `json:"log_retention_days,omitempty"`
	DependsOn        []string     `json:"depends_on,omitempty"`
	PostActions      []PostAction `json:"post_actions,omitempty"`
}

// PostAction is one post-build operation stored on the recipe.
type PostAction struct {
	ID         string `json:"id,omitempty"`
	Kind       string `json:"kind"`
	Enabled    *bool  `json:"enabled,omitempty"`
	EnvRecID   string `json:"env_rec_id,omitempty"`
	RemotePath string `json:"remote_path,omitempty"`
	Command    string `json:"command,omitempty"`
	When       string `json:"when,omitempty"`
	To         string `json:"to,omitempty"`
	Subject    string `json:"subject,omitempty"`
	AttachLog  *bool  `json:"attach_log,omitempty"`
}

const (
	// WhenSuccess runs after a successful job (default).
	WhenSuccess = "success"
	// WhenFailure runs after a failed job.
	WhenFailure = "failure"
	// WhenAlways runs after success or failure.
	WhenAlways = "always"
)

// CronPreview is the next N fire times for a cron expression.
type CronPreview struct {
	Expr  string   `json:"expr"`
	OK    bool     `json:"ok"`
	Error string   `json:"error,omitempty"`
	Times []string `json:"times,omitempty"`
	TZ    string   `json:"tz,omitempty"`
}

// ParseRecipe decodes recipe_json.
func ParseRecipe(raw string) Recipe {
	var r Recipe
	_ = json.Unmarshal([]byte(raw), &r)
	return r
}

// IsLocal reports that the job should use the checkout directory and not clone.
func (r Recipe) IsLocal() bool {
	return strings.EqualFold(strings.TrimSpace(r.SCMMode), "local")
}

// ShouldSync reports whether a git checkout should run before build.
// Missing auto_sync defaults to true so older recipes still pull.
func (r Recipe) ShouldSync() bool {
	if r.IsLocal() {
		return false
	}
	if r.AutoSync == nil {
		return true
	}
	return *r.AutoSync
}

// IsAbsLocalPath reports a host absolute directory (POSIX /... or Windows X:/...).
func IsAbsLocalPath(raw string) bool {
	p := strings.TrimSpace(filepath.ToSlash(raw))
	if p == "" || strings.Contains(p, "..") {
		return false
	}
	if strings.HasPrefix(p, "/") {
		return true
	}
	return len(p) >= 3 && p[1] == ':' && p[2] == '/'
}

// AbsLocalSource is the on-disk directory when scm_mode=local and the path is absolute.
// Relative local_source_path still lives under the workspace checkout.
func AbsLocalSource(r Recipe) string {
	if !r.IsLocal() {
		return ""
	}
	p := strings.TrimSpace(r.LocalSourcePath)
	if !IsAbsLocalPath(p) {
		return ""
	}
	return filepath.Clean(filepath.FromSlash(p))
}

// WorkRel is the checkout path under the workspace: project name, plus a relative local subdirectory.
// Absolute local_source_path is not folded in; callers should use AbsLocalSource for the real directory.
func WorkRel(name string, r Recipe) string {
	name = strings.TrimSpace(name)
	if AbsLocalSource(r) != "" {
		return name
	}
	extra := filepath.ToSlash(strings.TrimSpace(r.LocalSourcePath))
	extra = strings.Trim(extra, "/")
	if r.IsLocal() && extra != "" {
		if name == "" {
			return extra
		}
		return name + "/" + extra
	}
	return name
}

// CheckRepoURL accepts http(s), ssh, git, or file clone URLs.
func CheckRepoURL(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ErrRepoURL
	}
	lower := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(lower, "https://"),
		strings.HasPrefix(lower, "http://"),
		strings.HasPrefix(lower, "git@"),
		strings.HasPrefix(lower, "ssh://"),
		strings.HasPrefix(lower, "git://"),
		strings.HasPrefix(lower, "file://"):
		return nil
	default:
		return fmt.Errorf("%w: must be http(s), ssh, or git", ErrRepoURL)
	}
}

// CheckRepoBranch accepts a single git ref name such as main or release/V9.0.0-P01.
func CheckRepoBranch(raw string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || len(raw) > 200 {
		return ErrRepoBranch
	}
	if strings.Contains(raw, "..") || strings.Contains(raw, "//") || strings.HasPrefix(raw, "/") || strings.HasSuffix(raw, "/") {
		return ErrRepoBranch
	}
	for _, part := range strings.Split(raw, "/") {
		if part == "" || strings.HasPrefix(part, ".") || strings.HasSuffix(part, ".lock") {
			return ErrRepoBranch
		}
	}
	for _, r := range raw {
		if r <= 0x20 || r == 0x7f || strings.ContainsRune("\\:~^?*[@{", r) {
			return ErrRepoBranch
		}
	}
	return nil
}

// CheckCron accepts empty, 5-field cron, or robfig descriptors such as @daily.
func CheckCron(expr string) error {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return nil
	}
	if _, err := cronParser.Parse(expr); err != nil {
		return ErrCron
	}
	return nil
}

// PreviewCron returns the next n fire times in RFC3339 using the same parser as CheckCron.
func PreviewCron(expr string, n int) CronPreview {
	expr = strings.TrimSpace(expr)
	out := CronPreview{Expr: expr, TZ: time.Now().Location().String()}
	if expr == "" {
		return out
	}
	if n <= 0 {
		n = 5
	}
	if n > 10 {
		n = 10
	}
	sched, err := cronParser.Parse(expr)
	if err != nil {
		out.Error = ErrCron.Error()
		return out
	}
	now := time.Now()
	t := now
	for i := 0; i < n; i++ {
		t = sched.Next(t)
		if t.IsZero() {
			break
		}
		out.Times = append(out.Times, t.Format(time.RFC3339))
	}
	out.OK = len(out.Times) > 0
	if !out.OK {
		out.Error = ErrCron.Error()
	}
	return out
}

// ArtifactSpec is the output directory and filename glob used when collecting build outputs.
func ArtifactSpec(r Recipe, payloadArtifact string) (dir, pattern string) {
	payloadArtifact = strings.TrimSpace(payloadArtifact)
	if payloadArtifact != "" {
		slashed := filepath.ToSlash(payloadArtifact)
		base := filepath.Base(slashed)
		if strings.ContainsAny(base, "*?[") {
			dir = strings.Trim(filepath.ToSlash(filepath.Dir(slashed)), "/")
			if dir == "." {
				dir = ""
			}
			pattern = base
		} else {
			dir = slashed
		}
	}
	if dir == "" {
		dir = strings.TrimSpace(r.OutputDir)
	}
	if pattern == "" {
		pattern = strings.TrimSpace(r.ArtifactPattern)
	}
	if pattern == "" && len(r.Artifacts) > 0 {
		first := filepath.ToSlash(strings.TrimSpace(r.Artifacts[0]))
		base := filepath.Base(first)
		if strings.ContainsAny(base, "*?[") {
			if dir == "" {
				parent := strings.Trim(filepath.ToSlash(filepath.Dir(first)), "/")
				if parent != "." {
					dir = parent
				}
			}
			pattern = base
		} else if dir == "" {
			dir = first
		} else {
			pattern = first
		}
	}
	if dir == "" {
		dir = "target"
	}
	return dir, pattern
}

// CollectArtifacts lists files under work that match the recipe or job payload.
// An empty pattern keeps the historical default: *.jar and *.war in the output directory.
func CollectArtifacts(work string, r Recipe, payloadArtifact string) ([]string, error) {
	dir, pattern := ArtifactSpec(r, payloadArtifact)
	root := dir
	if !filepath.IsAbs(root) {
		if err := slug.CheckRelPath(dir); err != nil {
			return nil, err
		}
		root = filepath.Join(work, filepath.FromSlash(dir))
	}
	st, err := os.Stat(root)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return []string{root}, nil
	}
	recursive := strings.Contains(pattern, "**")
	match := basenamePattern(pattern)
	var out []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if path == root {
				return nil
			}
			if !recursive {
				return fs.SkipDir
			}
			return nil
		}
		if matchArtifact(d.Name(), match) {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%w in %s (%s)", ErrNoArtifacts, dir, displayPattern(pattern))
	}
	return out, nil
}

const maxPushFiles = 2000

func skipHiddenPushDir(name string, isDir bool) bool {
	return isDir && (name == ".git" || strings.HasPrefix(name, "."))
}

func hasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

func isRootGlob(pattern string) bool {
	p := strings.Trim(filepath.ToSlash(pattern), "/")
	return p == "" || p == "." || p == "*" || p == "**"
}

// expandPushRels 把精确路径与通配符展开为工程内相对路径；禁止匹配工程根。
func expandPushRels(work string, rels []string) ([]string, error) {
	var out []string
	seen := map[string]struct{}{}
	for _, raw := range rels {
		matches, err := expandPushRel(work, raw)
		if err != nil {
			return nil, err
		}
		for _, rel := range matches {
			if _, ok := seen[rel]; ok {
				continue
			}
			seen[rel] = struct{}{}
			out = append(out, rel)
		}
	}
	return out, nil
}

func expandPushRel(work, raw string) ([]string, error) {
	slashed := filepath.ToSlash(strings.TrimSpace(raw))
	slashed = strings.TrimPrefix(slashed, "./")
	slashed = strings.TrimPrefix(slashed, "/")
	if slashed == "" || slashed == "." || isRootGlob(slashed) {
		return nil, fmt.Errorf("%w: project root", ErrNoArtifacts)
	}
	if !hasGlobMeta(slashed) {
		_, rel, err := utils.Resolve(work, slashed)
		if err != nil {
			return nil, err
		}
		if rel == "" {
			return nil, fmt.Errorf("%w: project root", ErrNoArtifacts)
		}
		return []string{rel}, nil
	}
	if strings.Contains(slashed, "..") {
		return nil, utils.ErrOutside
	}
	re, err := compilePushGlob(slashed)
	if err != nil {
		return nil, err
	}
	walkRoot, err := globWalkRoot(work, slashed)
	if err != nil {
		return nil, err
	}
	if _, statErr := os.Stat(walkRoot); statErr != nil {
		if os.IsNotExist(statErr) {
			return nil, fmt.Errorf("%w: %s", ErrNoArtifacts, slashed)
		}
		return nil, statErr
	}
	var matched []string
	err = filepath.WalkDir(walkRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == work {
			return nil
		}
		if d.IsDir() && skipHiddenPushDir(d.Name(), true) {
			return fs.SkipDir
		}
		rel, relErr := filepath.Rel(work, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == "." || strings.HasPrefix(rel, "../") {
			return nil
		}
		if !re.MatchString(rel) {
			return nil
		}
		if len(matched) >= maxPushFiles {
			return fmt.Errorf("too many files to push (>%d)", maxPushFiles)
		}
		matched = append(matched, rel)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(matched) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNoArtifacts, slashed)
	}
	return matched, nil
}

func globWalkRoot(work, pattern string) (string, error) {
	i := strings.IndexAny(pattern, "*?[")
	prefix := pattern
	if i >= 0 {
		prefix = pattern[:i]
		if slash := strings.LastIndex(prefix, "/"); slash >= 0 {
			prefix = prefix[:slash]
		} else {
			prefix = ""
		}
	}
	if prefix == "" {
		return work, nil
	}
	dest, _, err := utils.Resolve(work, prefix)
	if err != nil {
		return "", err
	}
	return dest, nil
}

func compilePushGlob(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteByte('^')
	runes := []rune(pattern)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '*' && i+1 < len(runes) && runes[i+1] == '*' {
			if i+2 < len(runes) && runes[i+2] == '/' {
				b.WriteString("(?:.*/)?")
				i += 2
				continue
			}
			b.WriteString(".*")
			i++
			continue
		}
		switch runes[i] {
		case '*':
			b.WriteString("[^/]*")
		case '?':
			b.WriteString("[^/]")
		default:
			b.WriteString(regexp.QuoteMeta(string(runes[i])))
		}
	}
	b.WriteByte('$')
	return regexp.Compile(b.String())
}

func basenamePattern(pattern string) string {
	pattern = strings.TrimSpace(strings.ReplaceAll(filepath.ToSlash(pattern), "**", "*"))
	base := filepath.Base(pattern)
	if base == "." || base == "" {
		return "*"
	}
	return base
}

func matchArtifact(name, pattern string) bool {
	if pattern == "" {
		lower := strings.ToLower(name)
		return strings.HasSuffix(lower, ".jar") || strings.HasSuffix(lower, ".war")
	}
	if pattern == "*" {
		return true
	}
	ok, err := filepath.Match(pattern, name)
	return err == nil && ok
}

func displayPattern(pattern string) string {
	if pattern == "" {
		return "*.jar|*.war"
	}
	return pattern
}

// CheckPostAction accepts a batch email or shell post action.
func CheckPostAction(a PostAction) error {
	switch strings.ToLower(strings.TrimSpace(a.Kind)) {
	case "email":
		if strings.TrimSpace(a.EnvRecID) == "" {
			return ErrPostEnv
		}
		return nil
	case "shell":
		if strings.TrimSpace(a.Command) == "" {
			return ErrPostShell
		}
		return nil
	default:
		return ErrPostKind
	}
}

// UpsertPostAction replaces the first action of the same kind, or appends.
func UpsertPostAction(r Recipe, act PostAction) Recipe {
	act.Kind = strings.ToLower(strings.TrimSpace(act.Kind))
	act.When = ActionWhen(act)
	acts := r.Actions()
	out := make([]PostAction, 0, len(acts)+1)
	replaced := false
	for _, item := range acts {
		if !strings.EqualFold(item.Kind, act.Kind) {
			out = append(out, item)
			continue
		}
		if replaced {
			continue
		}
		if strings.TrimSpace(act.ID) == "" {
			act.ID = item.ID
		}
		out = append(out, act)
		replaced = true
	}
	if !replaced {
		out = append(out, act)
	}
	r.PostActions = out
	return r
}

// Actions returns explicit post-build operations, or a legacy FTP binding.
func (r Recipe) Actions() []PostAction {
	if len(r.PostActions) > 0 {
		return r.PostActions
	}
	if strings.TrimSpace(r.FTPEnvRecID) == "" {
		return nil
	}
	return []PostAction{{Kind: "ftp", EnvRecID: r.FTPEnvRecID, RemotePath: r.RemotePath}}
}

// ActionEnabled reports whether a post action should run. Missing enabled defaults to true.
func ActionEnabled(a PostAction) bool {
	if a.Enabled == nil {
		return true
	}
	return *a.Enabled
}

// ActionWhen normalizes when. Empty defaults to success.
func ActionWhen(a PostAction) string {
	switch strings.ToLower(strings.TrimSpace(a.When)) {
	case WhenFailure, "failed", "fail", "error":
		return WhenFailure
	case WhenAlways, "all":
		return WhenAlways
	default:
		return WhenSuccess
	}
}

// ActionMatches reports whether the action should run for a successful or failed job.
func ActionMatches(a PostAction, ok bool) bool {
	if !ActionEnabled(a) {
		return false
	}
	switch ActionWhen(a) {
	case WhenAlways:
		return true
	case WhenFailure:
		return !ok
	default:
		return ok
	}
}

// ActionAttachLog reports whether an email action opted into appending the job log.
func ActionAttachLog(a PostAction) bool {
	return a.AttachLog != nil && *a.AttachLog
}

// ShouldAttachMailLog reports whether a notify mail should include the job log.
// Failed jobs always attach the tail so recipients can diagnose; success follows attach_log.
func ShouldAttachMailLog(a PostAction, ok bool) bool {
	if !ok {
		return true
	}
	return ActionAttachLog(a)
}

// IsEmailAction reports smtp / email notify kinds.
func IsEmailAction(kind string) bool {
	k := strings.ToLower(strings.TrimSpace(kind))
	return k == "email" || k == "smtp"
}

// HasSelfDepend reports that depends_on includes the project itself.
func HasSelfDepend(ids []string, selfID string) bool {
	selfID = strings.TrimSpace(selfID)
	if selfID == "" {
		return false
	}
	for _, id := range ids {
		if strings.TrimSpace(id) == selfID {
			return true
		}
	}
	return false
}

// InDependChain reports that id already appears in the trigger chain (cycle).
func InDependChain(chain []string, id string) bool {
	id = strings.TrimSpace(id)
	if id == "" {
		return false
	}
	for _, item := range chain {
		if strings.TrimSpace(item) == id {
			return true
		}
	}
	return false
}
