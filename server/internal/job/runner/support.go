package runner

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"auto-pack/server/internal/host/git"
	"auto-pack/server/internal/job"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/workspace"
)

func (e *Engine) workDir(ctx context.Context, j job.Job, prj project.Project, p job.Payload) string {
	if p.CloneDir != "" {
		if filepath.IsAbs(p.CloneDir) {
			return p.CloneDir
		}
		return workspace.ProjectDir(e.wsRoot, e.workspaceCode(ctx, j.WorkspaceRecID), j.WorkspaceRecID, p.CloneDir)
	}
	rel := strings.TrimSpace(prj.WorkRelPath)
	if rel == "" {
		rel = project.WorkRel(prj.ProjectName, project.ParseRecipe(prj.RecipeJSON))
	}
	if rel == "" {
		rel = j.RecID
	}
	return workspace.ProjectDir(e.wsRoot, e.workspaceCode(ctx, j.WorkspaceRecID), j.WorkspaceRecID, rel)
}

func (e *Engine) workspaceCode(ctx context.Context, id string) string {
	if e.st == nil || id == "" {
		return ""
	}
	q := e.st.Rebind(`SELECT workspace_code FROM AP_WORKSPACE WHERE rec_id = ?`)
	var code string
	if err := e.st.SQL().QueryRowContext(ctx, q, id).Scan(&code); err != nil {
		return ""
	}
	return code
}

func (e *Engine) gitOpt(ctx context.Context, workspaceID string) git.Options {
	return e.gitOptFor(ctx, workspaceID, "")
}

func (e *Engine) gitOptFor(ctx context.Context, workspaceID, gitEnvID string) git.Options {
	return git.Options{Depth: e.depth, Auth: git.MapAuth(e.envCfg(ctx, gitEnvID, workspaceID, "git"))}
}

func (e *Engine) envCfg(ctx context.Context, recID, workspaceID, kind string) map[string]any {
	if recID != "" && e.envs != nil {
		_, cfg, err := e.envs.DecryptRow(ctx, recID)
		if err == nil {
			return cfg
		}
	}
	return e.kindCfg(ctx, workspaceID, kind)
}

func (e *Engine) kindCfg(ctx context.Context, workspaceID, kind string) map[string]any {
	if e.envs == nil {
		return nil
	}
	return e.envs.KindConfig(ctx, workspaceID, kind)
}

func mapString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func (e *Engine) openLog(jobID string) (io.Writer, func()) {
	dir := filepath.Join(e.dataDir, "jobs", jobID)
	_ = os.MkdirAll(dir, 0o755)
	f, err := os.OpenFile(filepath.Join(dir, "job.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return io.Discard, func() {}
	}
	return f, func() { _ = f.Close() }
}

// ReadLog returns job log bytes from offset. used is the actual start position (0 if offset is past EOF).
func (e *Engine) ReadLog(jobID string, offset int64) (string, int64, int64, error) {
	f, err := os.Open(filepath.Join(e.dataDir, "jobs", jobID, "job.log"))
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0, 0, nil
		}
		return "", 0, 0, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", 0, 0, err
	}
	size := info.Size()
	used := offset
	if used < 0 || used > size {
		used = 0
	}
	if used == size {
		return "", size, used, nil
	}
	if used > 0 {
		if _, err := f.Seek(used, io.SeekStart); err != nil {
			return "", 0, 0, err
		}
	}
	b, err := io.ReadAll(f)
	if err != nil {
		return "", 0, 0, err
	}
	return string(b), size, used, nil
}

func clockNow() string {
	return time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")
}

func logLine(w io.Writer, format string, args ...any) {
	if w == nil {
		return
	}
	fmt.Fprintf(w, "[%s] %s\n", clockNow(), fmt.Sprintf(format, args...))
}
