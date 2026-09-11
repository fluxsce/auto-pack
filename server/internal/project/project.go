package project

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/slug"
	"auto-pack/server/internal/store"
)

// Project is an engineering recipe.
type Project struct {
	RecID          string `json:"rec_id"`
	WorkspaceRecID string `json:"workspace_rec_id"`
	ProjectName    string `json:"project_name"`
	ProjectType    string `json:"project_type"`
	RepoURL        string `json:"repo_url,omitempty"`
	RepoBranch     string `json:"repo_branch,omitempty"`
	WorkRelPath    string `json:"work_rel_path,omitempty"`
	RecipeJSON     string `json:"recipe_json"`
	CronExpr       string `json:"cron_expr,omitempty"`
	IsEnabled      int    `json:"is_enabled"`
	Remark         string `json:"remark,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	LastJobStatus  string `json:"last_job_status,omitempty"`
	LastJobAt      string `json:"last_job_at,omitempty"`
}

// GitBranchUpdate is the result of setting repo_branch on git projects in one workspace.
type GitBranchUpdate struct {
	Updated int    `json:"updated"`
	Skipped int    `json:"skipped"`
	Branch  string `json:"branch"`
}

// PostActionUpdate is the result of upserting one post-build action on every project.
type PostActionUpdate struct {
	Updated int    `json:"updated"`
	Kind    string `json:"kind"`
}

const projectCols = `rec_id, workspace_rec_id, project_name, project_type, COALESCE(repo_url,''),
		COALESCE(repo_branch,''), COALESCE(work_rel_path,''), recipe_json, COALESCE(cron_expr,''), is_enabled, COALESCE(remark,''),
		COALESCE(created_at,''), COALESCE(updated_at,'')`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProject(s rowScanner) (Project, error) {
	var p Project
	err := s.Scan(&p.RecID, &p.WorkspaceRecID, &p.ProjectName, &p.ProjectType, &p.RepoURL,
		&p.RepoBranch, &p.WorkRelPath, &p.RecipeJSON, &p.CronExpr, &p.IsEnabled, &p.Remark,
		&p.CreatedAt, &p.UpdatedAt)
	return p, err
}

// Service manages projects. Writes call iam.Allow.
type Service struct {
	st    *store.Store
	guard *iam.Guard
}

// NewService returns a project Service.
func NewService(st *store.Store, guard *iam.Guard) *Service {
	return &Service{st: st, guard: guard}
}

// List returns projects in a workspace (any member).
func (s *Service) List(ctx context.Context, actor iam.Actor, workspaceID string) ([]Project, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return nil, err
	}
	list, err := s.ListRaw(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if err := s.attachLastJobs(ctx, workspaceID, list); err != nil {
		return nil, err
	}
	return list, nil
}

// ListRaw lists projects in a workspace without IAM (engine / dependents).
func (s *Service) ListRaw(ctx context.Context, workspaceID string) ([]Project, error) {
	q := s.st.Rebind(`SELECT ` + projectCols + ` FROM AP_PROJECT WHERE workspace_rec_id = ? ORDER BY project_name`)
	rows, err := s.st.SQL().QueryContext(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get loads one project after a read check.
func (s *Service) Get(ctx context.Context, actor iam.Actor, id string) (Project, error) {
	p, err := s.GetRaw(ctx, id)
	if err != nil {
		return Project{}, err
	}
	if err := s.guard.CanRead(ctx, actor, p.WorkspaceRecID); err != nil {
		return Project{}, err
	}
	return p, nil
}

// GetRaw loads a project without IAM (engine / cron).
func (s *Service) GetRaw(ctx context.Context, id string) (Project, error) {
	q := s.st.Rebind(`SELECT ` + projectCols + ` FROM AP_PROJECT WHERE rec_id = ?`)
	p, err := scanProject(s.st.SQL().QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, err
	}
	return p, err
}

// ListEnabledCron returns enabled projects that have cron_expr.
func (s *Service) ListEnabledCron(ctx context.Context) ([]Project, error) {
	q := `SELECT ` + projectCols + ` FROM AP_PROJECT WHERE is_enabled = 1 AND cron_expr IS NOT NULL AND cron_expr <> ''`
	rows, err := s.st.SQL().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Project
	for rows.Next() {
		p, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Save creates or updates a project (project.write).
func (s *Service) Save(ctx context.Context, actor iam.Actor, p Project) (Project, error) {
	if err := s.guard.Allow(ctx, actor, p.WorkspaceRecID, iam.ActionProjectWrite); err != nil {
		return Project{}, err
	}
	p.ProjectName = strings.TrimSpace(p.ProjectName)
	p.ProjectType = strings.TrimSpace(p.ProjectType)
	if p.ProjectName == "" || p.ProjectType == "" {
		return Project{}, errors.New("project_name and project_type required")
	}
	if err := slug.CheckSlug(p.ProjectName); err != nil {
		return Project{}, err
	}
	recipe := ParseRecipe(p.RecipeJSON)
	if !recipe.IsLocal() {
		if err := slug.CheckRelPath(recipe.LocalSourcePath); err != nil {
			return Project{}, err
		}
	}
	if recipe.IsLocal() {
		p.RepoURL = strings.TrimSpace(p.RepoURL)
	} else if strings.EqualFold(strings.TrimSpace(recipe.SCMMode), "git") {
		if err := CheckRepoURL(p.RepoURL); err != nil {
			return Project{}, err
		}
	}
	if err := CheckCron(p.CronExpr); err != nil {
		return Project{}, err
	}
	if HasSelfDepend(recipe.DependsOn, p.RecID) {
		return Project{}, errors.New("depends_on cannot include self")
	}
	if strings.TrimSpace(p.WorkRelPath) == "" {
		p.WorkRelPath = WorkRel(p.ProjectName, recipe)
	}
	if err := slug.CheckRelPath(p.WorkRelPath); err != nil {
		return Project{}, err
	}
	if p.RecipeJSON == "" {
		raw, _ := json.Marshal(Recipe{Command: "echo missing recipe"})
		p.RecipeJSON = string(raw)
	}
	if p.IsEnabled == 0 && p.RecID == "" {
		p.IsEnabled = 1
	}
	if p.RecID == "" {
		p.RecID = access.NewID()
		ins := s.st.Rebind(`INSERT INTO AP_PROJECT (rec_id, workspace_rec_id, project_name, project_type, repo_url, repo_branch,
			work_rel_path, recipe_json, cron_expr, is_enabled, remark)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		_, err := s.st.SQL().ExecContext(ctx, ins, p.RecID, p.WorkspaceRecID, p.ProjectName, p.ProjectType,
			nullStr(p.RepoURL), nullStr(p.RepoBranch), nullStr(p.WorkRelPath), p.RecipeJSON, nullStr(p.CronExpr), p.IsEnabled, nullStr(p.Remark))
		return p, err
	}
	upd := s.st.Rebind(`UPDATE AP_PROJECT SET project_name=?, project_type=?, repo_url=?, repo_branch=?, work_rel_path=?,
		recipe_json=?, cron_expr=?, is_enabled=?, remark=? WHERE rec_id=?`)
	_, err := s.st.SQL().ExecContext(ctx, upd, p.ProjectName, p.ProjectType, nullStr(p.RepoURL), nullStr(p.RepoBranch),
		nullStr(p.WorkRelPath), p.RecipeJSON, nullStr(p.CronExpr), p.IsEnabled, nullStr(p.Remark), p.RecID)
	return p, err
}

// UpdateGitBranches sets repo_branch on git projects in a workspace (project.write).
// Local-directory recipes are skipped.
func (s *Service) UpdateGitBranches(ctx context.Context, actor iam.Actor, workspaceID, branch string) (GitBranchUpdate, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	branch = strings.TrimSpace(branch)
	if err := CheckRepoBranch(branch); err != nil {
		return GitBranchUpdate{}, err
	}
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionProjectWrite); err != nil {
		return GitBranchUpdate{}, err
	}
	list, err := s.ListRaw(ctx, workspaceID)
	if err != nil {
		return GitBranchUpdate{}, err
	}
	ids := make([]string, 0, len(list))
	skipped := 0
	for _, p := range list {
		if ParseRecipe(p.RecipeJSON).IsLocal() {
			skipped++
			continue
		}
		ids = append(ids, p.RecID)
	}
	out := GitBranchUpdate{Skipped: skipped, Branch: branch}
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(ids)), ",")
	q := s.st.Rebind(`UPDATE AP_PROJECT SET repo_branch=? WHERE workspace_rec_id=? AND rec_id IN (` + placeholders + `)`)
	args := make([]any, 0, 2+len(ids))
	args = append(args, branch, workspaceID)
	for _, id := range ids {
		args = append(args, id)
	}
	if _, err := s.st.SQL().ExecContext(ctx, q, args...); err != nil {
		return GitBranchUpdate{}, err
	}
	out.Updated = len(ids)
	return out, nil
}

// UpdatePostActions upserts one email or shell post-build action on every project (project.write).
func (s *Service) UpdatePostActions(ctx context.Context, actor iam.Actor, workspaceID string, act PostAction) (PostActionUpdate, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if err := CheckPostAction(act); err != nil {
		return PostActionUpdate{}, err
	}
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionProjectWrite); err != nil {
		return PostActionUpdate{}, err
	}
	list, err := s.ListRaw(ctx, workspaceID)
	if err != nil {
		return PostActionUpdate{}, err
	}
	out := PostActionUpdate{Kind: strings.ToLower(strings.TrimSpace(act.Kind))}
	q := s.st.Rebind(`UPDATE AP_PROJECT SET recipe_json=? WHERE rec_id=?`)
	for _, p := range list {
		recipe := UpsertPostAction(ParseRecipe(p.RecipeJSON), act)
		for i := range recipe.PostActions {
			if strings.TrimSpace(recipe.PostActions[i].ID) == "" {
				recipe.PostActions[i].ID = access.NewID()
			}
		}
		raw, err := json.Marshal(recipe)
		if err != nil {
			return PostActionUpdate{}, err
		}
		if _, err := s.st.SQL().ExecContext(ctx, q, string(raw), p.RecID); err != nil {
			return PostActionUpdate{}, err
		}
		out.Updated++
	}
	return out, nil
}

// UpdateCommands rewrites recipe.command on matching projects (project.write).
func (s *Service) UpdateCommands(ctx context.Context, actor iam.Actor, workspaceID string, patch CommandPatch) (CommandUpdate, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if err := CheckCommandPatch(patch); err != nil {
		return CommandUpdate{}, err
	}
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionProjectWrite); err != nil {
		return CommandUpdate{}, err
	}
	list, err := s.ListRaw(ctx, workspaceID)
	if err != nil {
		return CommandUpdate{}, err
	}
	family := strings.ToLower(strings.TrimSpace(patch.Family))
	if family == "all" {
		family = ""
	}
	out := CommandUpdate{Family: family}
	q := s.st.Rebind(`UPDATE AP_PROJECT SET recipe_json=? WHERE rec_id=?`)
	for _, p := range list {
		if !MatchCommandFamily(p.ProjectType, family) {
			out.Skipped++
			continue
		}
		recipe, ok := PatchRecipe(ParseRecipe(p.RecipeJSON), patch)
		if !ok {
			out.Skipped++
			continue
		}
		raw, err := json.Marshal(recipe)
		if err != nil {
			return CommandUpdate{}, err
		}
		if _, err := s.st.SQL().ExecContext(ctx, q, string(raw), p.RecID); err != nil {
			return CommandUpdate{}, err
		}
		out.Updated++
	}
	return out, nil
}

// SetRepoBranch writes repo_branch without IAM (runner after branch_clone).
func (s *Service) SetRepoBranch(ctx context.Context, id, branch string) error {
	id = strings.TrimSpace(id)
	branch = strings.TrimSpace(branch)
	if id == "" {
		return errors.New("project id required")
	}
	if err := CheckRepoBranch(branch); err != nil {
		return err
	}
	q := s.st.Rebind(`UPDATE AP_PROJECT SET repo_branch=? WHERE rec_id=?`)
	_, err := s.st.SQL().ExecContext(ctx, q, branch, id)
	return err
}

// Delete removes a project (project.write).
func (s *Service) Delete(ctx context.Context, actor iam.Actor, id string) error {
	p, err := s.GetRaw(ctx, id)
	if err != nil {
		return err
	}
	if err := s.guard.Allow(ctx, actor, p.WorkspaceRecID, iam.ActionProjectWrite); err != nil {
		return err
	}
	q := s.st.Rebind(`DELETE FROM AP_PROJECT WHERE rec_id = ?`)
	_, err = s.st.SQL().ExecContext(ctx, q, id)
	return err
}

// Clone 在同一空间复制工程配方，生成新的 slug 名称与检出目录。不复制作业。
func (s *Service) Clone(ctx context.Context, actor iam.Actor, id string) (Project, error) {
	src, err := s.GetRaw(ctx, id)
	if err != nil {
		return Project{}, err
	}
	names, err := s.namesInWorkspace(ctx, src.WorkspaceRecID)
	if err != nil {
		return Project{}, err
	}
	src.RecID = ""
	src.ProjectName = uniqueCopyName(src.ProjectName, names)
	src.WorkRelPath = ""
	src.CreatedAt = ""
	src.UpdatedAt = ""
	src.LastJobStatus = ""
	src.LastJobAt = ""
	return s.Save(ctx, actor, src)
}

func (s *Service) attachLastJobs(ctx context.Context, workspaceID string, list []Project) error {
	if len(list) == 0 {
		return nil
	}
	q := s.st.Rebind(`SELECT COALESCE(project_rec_id,''), job_status, COALESCE(finished_at,''), COALESCE(started_at,''), created_at
		FROM AP_JOB WHERE workspace_rec_id = ? AND job_type = 'build' ORDER BY created_at DESC`)
	rows, err := s.st.SQL().QueryContext(ctx, q, workspaceID)
	if err != nil {
		return err
	}
	defer rows.Close()
	idx := make(map[string]int, len(list))
	for i := range list {
		idx[list[i].RecID] = i
	}
	seen := make(map[string]struct{}, len(list))
	for rows.Next() {
		var pid, status, finished, started, created string
		if err := rows.Scan(&pid, &status, &finished, &started, &created); err != nil {
			return err
		}
		if pid == "" {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		i, ok := idx[pid]
		if !ok {
			continue
		}
		seen[pid] = struct{}{}
		list[i].LastJobStatus = status
		list[i].LastJobAt = firstNonEmpty(finished, started, created)
		if len(seen) == len(list) {
			break
		}
	}
	return rows.Err()
}

func (s *Service) namesInWorkspace(ctx context.Context, workspaceID string) (map[string]struct{}, error) {
	q := s.st.Rebind(`SELECT project_name FROM AP_PROJECT WHERE workspace_rec_id = ?`)
	rows, err := s.st.SQL().QueryContext(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]struct{}{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out[name] = struct{}{}
	}
	return out, rows.Err()
}

func uniqueCopyName(name string, taken map[string]struct{}) string {
	base := strings.TrimSpace(name)
	if base == "" {
		base = "project"
	}
	const extra = 7
	if len(base) > 64-extra {
		base = strings.TrimRight(base[:64-extra], "-_")
	}
	try := func(n string) bool {
		if slug.CheckSlug(n) != nil {
			return false
		}
		_, used := taken[n]
		return !used
	}
	candidate := base + "-copy"
	if try(candidate) {
		return candidate
	}
	for i := 2; i < 100; i++ {
		candidate = fmt.Sprintf("%s-copy%d", base, i)
		if try(candidate) {
			return candidate
		}
	}
	return "copy-" + access.NewID()[:8]
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func nullStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
