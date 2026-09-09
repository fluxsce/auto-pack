// Package migrate 启动时把 script/db/{engine}/init.sql 打进平台库。
package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"auto-pack/server/internal/store"
)

// Apply runs init.sql then 98_patch.sql for the current driver.
func Apply(ctx context.Context, st *store.Store, driver string) (string, error) {
	path, err := FindInitSQL(driver)
	if err != nil {
		return "", err
	}
	if err := ApplyFile(ctx, st, path); err != nil {
		return path, err
	}
	patch := filepath.Join(filepath.Dir(path), "98_patch.sql")
	if _, err := os.Stat(patch); err == nil {
		if err := ApplyPatchFile(ctx, st, patch); err != nil {
			return patch, err
		}
	}
	return path, nil
}

// ApplyFile executes statements in path.
func ApplyFile(ctx context.Context, st *store.Store, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("migrate: read %s: %w", path, err)
	}
	stmts := splitSQL(string(raw))
	if len(stmts) == 0 {
		return fmt.Errorf("migrate: %s has no statements", path)
	}
	for i, q := range stmts {
		if err := st.ExecRaw(ctx, q); err != nil {
			head := q
			if len(head) > 160 {
				head = head[:160] + "..."
			}
			return fmt.Errorf("migrate: %s statement %d: %w\n%s", path, i+1, err, head)
		}
	}
	return nil
}

// ApplyPatchFile runs additive patches; duplicate column/index is ignored.
func ApplyPatchFile(ctx context.Context, st *store.Store, path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("migrate: read %s: %w", path, err)
	}
	for i, q := range splitSQL(string(raw)) {
		if err := st.ExecRaw(ctx, q); err != nil {
			if isDupOrNoop(err) {
				continue
			}
			return fmt.Errorf("migrate: %s statement %d: %w", path, i+1, err)
		}
	}
	return nil
}

// FindInitSQL looks for script/db/{driver}/init.sql near cwd or the executable.
func FindInitSQL(driver string) (string, error) {
	name := store.NormalizeDriver(driver)
	if name == "" {
		return "", fmt.Errorf("migrate: driver is required")
	}
	rel := filepath.Join("script", "db", name, "init.sql")
	var candidates []string
	candidates = append(candidates, rel, filepath.Join("..", rel), filepath.Join("../..", rel))
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		candidates = append(candidates, filepath.Join(dir, rel), filepath.Join(dir, "..", rel))
	}
	if wd, err := os.Getwd(); err == nil {
		cur := wd
		for i := 0; i < 6; i++ {
			candidates = append(candidates, filepath.Join(cur, rel))
			parent := filepath.Dir(cur)
			if parent == cur {
				break
			}
			cur = parent
		}
	}
	seen := map[string]struct{}{}
	for _, c := range candidates {
		c = filepath.Clean(c)
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		st, err := os.Stat(c)
		if err == nil && !st.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("migrate: %s not found", rel)
}

func splitSQL(raw string) []string {
	var out []string
	var b strings.Builder
	for _, line := range strings.Split(raw, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "--") {
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
		if strings.HasSuffix(trim, ";") {
			stmt := strings.TrimSpace(b.String())
			b.Reset()
			if stmt != "" && stmt != ";" {
				out = append(out, strings.TrimSuffix(stmt, ";"))
			}
		}
	}
	if tail := strings.TrimSpace(b.String()); tail != "" {
		out = append(out, tail)
	}
	return out
}

func isDupOrNoop(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "duplicate") ||
		strings.Contains(s, "already exists") ||
		strings.Contains(s, "duplicate column") ||
		strings.Contains(s, "duplicate key name")
}
