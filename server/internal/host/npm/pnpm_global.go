package npm

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// 多个 Node/Vue 工程并行构建时都会执行 npm install -g pnpm，
// 同时改写同一套 Node 的全局目录会 ENOTEMPTY，随后 pnpm 不在 PATH。
var pnpmGlobalMu sync.Mutex

var npmInstallVerb = regexp.MustCompile(`(?i)^\s*npm\s+(?:install|i|add)\b`)

func ensurePnpm(ctx context.Context, command string, env []string, w io.Writer) (string, error) {
	spec, touched := globalPnpmSpec(command)
	if !touched {
		return command, nil
	}
	rewritten, _ := stripGlobalPnpmInstall(command)
	if err := installGlobalPnpmOnce(ctx, env, w, spec); err != nil {
		return "", err
	}
	if strings.TrimSpace(rewritten) == "" {
		return "", fmt.Errorf("构建命令去掉全局安装 pnpm 后为空")
	}
	return rewritten, nil
}

func installGlobalPnpmOnce(ctx context.Context, env []string, w io.Writer, spec string) error {
	pnpmGlobalMu.Lock()
	defer pnpmGlobalMu.Unlock()
	pkg := pnpmInstallArg(spec)
	if ver, ok := pnpmSatisfied(ctx, env, spec); ok {
		if spec == "" {
			logPnpm(w, "pnpm "+ver+" 已在 PATH，跳过 npm install -g pnpm")
		} else {
			logPnpm(w, "pnpm "+ver+" 已满足 "+pkg+"，跳过全局安装")
		}
		return nil
	}
	logPnpm(w, "全局安装 "+pkg+"（并行构建共用这一次）")
	if err := bashInstallPnpm(ctx, env, w, pkg); err == nil {
		if _, ok := pnpmSatisfied(ctx, env, spec); ok {
			return nil
		}
	}
	logPnpm(w, "全局安装 pnpm 失败，清理残留后重试一次")
	if err := clearStaleGlobalPnpm(ctx, env); err != nil {
		return err
	}
	if err := bashInstallPnpm(ctx, env, w, pkg); err != nil {
		return err
	}
	if _, ok := pnpmSatisfied(ctx, env, spec); !ok {
		return fmt.Errorf("npm install -g %s 完成后版本仍不满足", pkg)
	}
	return nil
}

func pnpmInstallArg(spec string) string {
	spec = strings.TrimPrefix(strings.TrimSpace(spec), "v")
	if spec == "" || strings.EqualFold(spec, "latest") {
		return "pnpm"
	}
	return "pnpm@" + spec
}

func logPnpm(w io.Writer, line string) {
	if w == nil {
		return
	}
	fmt.Fprintln(w, line)
}

func bashInstallPnpm(ctx context.Context, env []string, w io.Writer, pkg string) error {
	return execEnv(ctx, env, w, "npm", "install", "-g", pkg)
}

func clearStaleGlobalPnpm(ctx context.Context, env []string) error {
	prefix, err := npmGlobalPrefix(ctx, env)
	if err != nil {
		return err
	}
	for _, modules := range []string{
		filepath.Join(prefix, "lib", "node_modules"),
		filepath.Join(prefix, "node_modules"),
	} {
		_ = os.RemoveAll(filepath.Join(modules, "pnpm"))
		entries, err := os.ReadDir(modules)
		if err != nil {
			continue
		}
		for _, ent := range entries {
			if strings.HasPrefix(ent.Name(), ".pnpm-") {
				_ = os.RemoveAll(filepath.Join(modules, ent.Name()))
			}
		}
	}
	return nil
}

func npmGlobalPrefix(ctx context.Context, env []string) (string, error) {
	cmd := exec.CommandContext(ctx, "npm", "prefix", "-g")
	cmd.Env = mergedEnv(env)
	out, err := cmd.Output()
	prefix := strings.TrimSpace(string(out))
	if err != nil {
		return "", fmt.Errorf("npm prefix -g: %w", err)
	}
	if prefix == "" {
		return "", fmt.Errorf("npm prefix -g 为空")
	}
	return prefix, nil
}

func execEnv(ctx context.Context, env []string, w io.Writer, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = mergedEnv(env)
	out := w
	if out == nil {
		out = io.Discard
	}
	cmd.Stdout = out
	cmd.Stderr = out
	return cmd.Run()
}

func mergedEnv(extra []string) []string {
	if len(extra) == 0 {
		return os.Environ()
	}
	return append(append([]string{}, os.Environ()...), extra...)
}

func pnpmOnPath(extra []string) bool {
	return pnpmBin(extra) != ""
}

func pnpmBin(extra []string) string {
	pathVal := ""
	for _, ent := range mergedEnv(extra) {
		key, val, ok := strings.Cut(ent, "=")
		if ok && strings.EqualFold(key, "PATH") {
			pathVal = val
		}
	}
	if pathVal == "" {
		return ""
	}
	for _, dir := range filepath.SplitList(pathVal) {
		if dir == "" {
			continue
		}
		path := filepath.Join(dir, "pnpm")
		st, err := os.Stat(path)
		if err == nil && !st.IsDir() && st.Mode()&0o111 != 0 {
			return path
		}
	}
	return ""
}

func pnpmSatisfied(ctx context.Context, env []string, spec string) (string, bool) {
	bin := pnpmBin(env)
	if bin == "" {
		return "", false
	}
	if spec == "" || strings.EqualFold(spec, "latest") {
		ver, _ := pnpmVersion(ctx, env, bin)
		if ver == "" {
			ver = "已安装"
		}
		return ver, true
	}
	ver, err := pnpmVersion(ctx, env, bin)
	if err != nil || !versionSatisfied(ver, spec) {
		return ver, false
	}
	return ver, true
}

func pnpmVersion(ctx context.Context, env []string, bin string) (string, error) {
	cmd := exec.CommandContext(ctx, bin, "-v")
	cmd.Env = mergedEnv(env)
	out, err := cmd.Output()
	ver := strings.TrimSpace(string(out))
	if i := strings.IndexAny(ver, "\r\n"); i >= 0 {
		ver = strings.TrimSpace(ver[:i])
	}
	return ver, err
}

func versionSatisfied(installed, want string) bool {
	installed = cleanVer(installed)
	want = cleanVer(want)
	if want == "" || strings.EqualFold(want, "latest") {
		return installed != ""
	}
	got := strings.Split(installed, ".")
	need := strings.Split(want, ".")
	if len(need) > len(got) {
		return false
	}
	for i, part := range need {
		if !numericVer(part) || got[i] != part {
			return false
		}
	}
	return true
}

func cleanVer(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	if i := strings.IndexByte(v, '+'); i >= 0 {
		v = v[:i]
	}
	return v
}

func numericVer(part string) bool {
	if part == "" {
		return false
	}
	for _, r := range part {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func globalPnpmSpec(script string) (string, bool) {
	script = strings.ReplaceAll(script, "\r\n", "\n")
	spec := ""
	found := false
	for _, line := range strings.Split(script, "\n") {
		parts, _ := splitShellSeq(line)
		for _, part := range parts {
			if !isGlobalPnpmInstall(part) {
				continue
			}
			found = true
			if s := pnpmVersionSpec(part); s != "" {
				spec = s
			}
		}
	}
	return spec, found
}

func pnpmVersionSpec(seg string) string {
	for _, field := range strings.Fields(seg) {
		field = strings.Trim(field, `"'`)
		base, ver, has := strings.Cut(field, "@")
		if !strings.EqualFold(base, "pnpm") {
			continue
		}
		if !has {
			return ""
		}
		return strings.TrimPrefix(strings.TrimSpace(ver), "v")
	}
	return ""
}

func stripGlobalPnpmInstall(script string) (string, bool) {
	script = strings.ReplaceAll(script, "\r\n", "\n")
	lines := strings.Split(script, "\n")
	changed := false
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		next, drop := stripGlobalPnpmLine(line)
		if drop {
			changed = true
		}
		if strings.TrimSpace(next) == "" {
			continue
		}
		out = append(out, next)
	}
	return strings.Join(out, "\n"), changed
}

func stripGlobalPnpmLine(line string) (string, bool) {
	parts, seps := splitShellSeq(line)
	kept := make([]string, 0, len(parts))
	keptSeps := make([]string, 0, len(seps))
	dropped := false
	for i, part := range parts {
		if isGlobalPnpmInstall(part) {
			dropped = true
			continue
		}
		if strings.TrimSpace(part) == "" {
			continue
		}
		if len(kept) > 0 {
			sep := " && "
			if i > 0 && i-1 < len(seps) {
				sep = seps[i-1]
			}
			keptSeps = append(keptSeps, sep)
		}
		kept = append(kept, strings.TrimSpace(part))
	}
	if !dropped {
		return line, false
	}
	var b strings.Builder
	for i, part := range kept {
		if i > 0 {
			b.WriteString(keptSeps[i-1])
		}
		b.WriteString(part)
	}
	return b.String(), true
}

func splitShellSeq(line string) (parts []string, seps []string) {
	var b strings.Builder
	quote := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			b.WriteByte(c)
			if c == quote {
				quote = 0
			}
			continue
		}
		if c == '\'' || c == '"' {
			quote = c
			b.WriteByte(c)
			continue
		}
		if c == ';' {
			parts = append(parts, b.String())
			seps = append(seps, "; ")
			b.Reset()
			continue
		}
		if c == '&' && i+1 < len(line) && line[i+1] == '&' {
			parts = append(parts, b.String())
			seps = append(seps, " && ")
			b.Reset()
			i++
			continue
		}
		b.WriteByte(c)
	}
	parts = append(parts, b.String())
	return parts, seps
}

func isGlobalPnpmInstall(seg string) bool {
	seg = strings.TrimSpace(seg)
	if !npmInstallVerb.MatchString(seg) {
		return false
	}
	lower := strings.ToLower(seg)
	if !strings.Contains(lower, " -g") && !strings.Contains(lower, "--global") {
		return false
	}
	for _, field := range strings.Fields(seg) {
		field = strings.Trim(field, `"'`)
		base := field
		if i := strings.IndexByte(base, '@'); i > 0 {
			base = base[:i]
		}
		if strings.EqualFold(base, "pnpm") {
			return true
		}
	}
	return false
}
