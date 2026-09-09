package workspace

import (
	"context"
	"errors"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/store"
)

// Export 导出空间及其工程定义。不含环境密文、成员与作业。
func (s *Service) Export(ctx context.Context, actor iam.Actor, id string) (Bundle, error) {
	if err := s.guard.CanRead(ctx, actor, id); err != nil {
		return Bundle{}, err
	}
	ws, err := s.getForActor(ctx, actor, id)
	if err != nil {
		return Bundle{}, err
	}
	q := s.st.Rebind(`SELECT rec_id, project_name, project_type, COALESCE(repo_url,''), COALESCE(repo_branch,''),
		COALESCE(work_rel_path,''), recipe_json, COALESCE(cron_expr,''), is_enabled, COALESCE(remark,'')
		FROM AP_PROJECT WHERE workspace_rec_id = ? ORDER BY project_name`)
	rows, err := s.st.SQL().QueryContext(ctx, q, id)
	if err != nil {
		return Bundle{}, err
	}
	defer rows.Close()
	var projects []BundleProject
	for rows.Next() {
		var p BundleProject
		if err := rows.Scan(&p.SourceRecID, &p.ProjectName, &p.ProjectType, &p.RepoURL, &p.RepoBranch,
			&p.WorkRelPath, &p.RecipeJSON, &p.CronExpr, &p.IsEnabled, &p.Remark); err != nil {
			return Bundle{}, err
		}
		projects = append(projects, p)
	}
	if err := rows.Err(); err != nil {
		return Bundle{}, err
	}
	return stampBundle(Bundle{
		SourceWorkspaceRecID: ws.RecID,
		Workspace: BundleWorkspace{
			WorkspaceCode: ws.WorkspaceCode,
			WorkspaceName: ws.WorkspaceName,
			Remark:        ws.Remark,
		},
		Projects: projects,
	}), nil
}

// Import 从 v1/v2 包创建空间或替换目标空间的工程。
func (s *Service) Import(ctx context.Context, actor iam.Actor, req ImportRequest) (ImportResult, error) {
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = "new"
	}
	if mode != "new" && mode != "replace" {
		return ImportResult{}, ErrImportMode
	}
	if err := req.Bundle.Validate(); err != nil {
		return ImportResult{}, err
	}
	format := req.Bundle.resolvedFormat()
	if mode == "new" {
		return s.importNew(ctx, actor, req, format)
	}
	return s.importReplace(ctx, actor, req, format)
}

func (s *Service) importNew(ctx context.Context, actor iam.Actor, req ImportRequest, format int) (ImportResult, error) {
	code := firstNonEmpty(strings.TrimSpace(req.WorkspaceCode), req.Bundle.Workspace.WorkspaceCode)
	name := firstNonEmpty(strings.TrimSpace(req.WorkspaceName), req.Bundle.Workspace.WorkspaceName, code)
	if code == "" || name == "" {
		return ImportResult{}, errors.New("workspace_code and workspace_name required")
	}
	if err := CheckCode(code); err != nil {
		return ImportResult{}, err
	}
	if err := CheckDisplayName(name); err != nil {
		return ImportResult{}, err
	}
	if err := s.ensureCodeFree(ctx, code); err != nil {
		return ImportResult{}, err
	}
	id := access.NewID()
	n, err := s.insertBundle(ctx, actor, id, code, name, req.Bundle)
	if err != nil {
		return ImportResult{}, mapUniqueCode(err)
	}
	if err := mkdirWorkspace(s.root, code, id); err != nil {
		return ImportResult{}, err
	}
	ws, err := s.getForActor(ctx, actor, id)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Workspace: ws, ProjectsImported: n, Mode: "new", FormatVersion: format}, nil
}

func (s *Service) importReplace(ctx context.Context, actor iam.Actor, req ImportRequest, format int) (ImportResult, error) {
	target := strings.TrimSpace(req.TargetWorkspaceID)
	if target == "" {
		return ImportResult{}, errors.New("target_workspace_rec_id required")
	}
	if err := s.guard.Allow(ctx, actor, target, iam.ActionUserAdmin); err != nil {
		return ImportResult{}, err
	}
	busy := s.st.Rebind(`SELECT COUNT(1) FROM AP_JOB WHERE workspace_rec_id = ? AND job_status IN ('queued','running')`)
	var n int
	if err := s.st.SQL().QueryRowContext(ctx, busy, target).Scan(&n); err != nil {
		return ImportResult{}, err
	}
	if n > 0 {
		return ImportResult{}, ErrWorkspaceBusy
	}
	imported := 0
	err := s.st.Tx(ctx, func(tx *store.Store) error {
		steps := tx.Rebind(`DELETE FROM AP_JOB_STEP WHERE job_rec_id IN (SELECT rec_id FROM AP_JOB WHERE workspace_rec_id = ?)`)
		if _, err := tx.SQL().ExecContext(ctx, steps, target); err != nil {
			return err
		}
		jobs := tx.Rebind(`DELETE FROM AP_JOB WHERE workspace_rec_id = ?`)
		if _, err := tx.SQL().ExecContext(ctx, jobs, target); err != nil {
			return err
		}
		prjs := tx.Rebind(`DELETE FROM AP_PROJECT WHERE workspace_rec_id = ?`)
		if _, err := tx.SQL().ExecContext(ctx, prjs, target); err != nil {
			return err
		}
		count, err := insertBundleProjects(ctx, tx, target, req.Bundle)
		imported = count
		return err
	})
	if err != nil {
		return ImportResult{}, err
	}
	ws, err := s.getForActor(ctx, actor, target)
	if err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Workspace: ws, ProjectsImported: imported, Mode: "replace", FormatVersion: format}, nil
}

func (s *Service) insertBundle(ctx context.Context, actor iam.Actor, id, code, name string, bundle Bundle) (int, error) {
	imported := 0
	err := s.st.Tx(ctx, func(tx *store.Store) error {
		ins := tx.Rebind(`INSERT INTO AP_WORKSPACE (rec_id, workspace_code, workspace_name, owner_user_id, remark)
			VALUES (?, ?, ?, ?, ?)`)
		if _, err := tx.SQL().ExecContext(ctx, ins, id, code, name, actor.UserID, nullStr(bundle.Workspace.Remark)); err != nil {
			return mapUniqueCode(err)
		}
		mem := tx.Rebind(`INSERT INTO AP_WORKSPACE_MEMBER (rec_id, workspace_rec_id, user_rec_id, member_role)
			VALUES (?, ?, ?, ?)`)
		if _, err := tx.SQL().ExecContext(ctx, mem, access.NewID(), id, actor.UserID, iam.RoleOwner); err != nil {
			return err
		}
		n, err := insertBundleProjects(ctx, tx, id, bundle)
		imported = n
		return err
	})
	return imported, err
}

func insertBundleProjects(ctx context.Context, tx *store.Store, workspaceID string, bundle Bundle) (int, error) {
	idMap := map[string]string{}
	type pending struct {
		id, name, kind, repo, branch, work, recipe, cron, remark string
		enabled                                                  int
	}
	rows := make([]pending, 0, len(bundle.Projects))
	for _, src := range bundle.Projects {
		name := strings.TrimSpace(src.ProjectName)
		if name == "" {
			continue
		}
		if err := CheckSlug(name); err != nil {
			return 0, err
		}
		kind := strings.TrimSpace(src.ProjectType)
		if kind == "" {
			kind = "custom"
		}
		recipe, cron, work := src.toRecipe()
		id := access.NewID()
		if src.SourceRecID != "" {
			idMap[src.SourceRecID] = id
		}
		enabled := src.IsEnabled
		if enabled != 0 {
			enabled = 1
		}
		rows = append(rows, pending{
			id: id, name: name, kind: kind, repo: src.RepoURL, branch: src.RepoBranch,
			work: work, recipe: recipe, cron: cron, remark: src.Remark, enabled: enabled,
		})
	}
	ins := tx.Rebind(`INSERT INTO AP_PROJECT (rec_id, workspace_rec_id, project_name, project_type, repo_url, repo_branch,
		work_rel_path, recipe_json, cron_expr, is_enabled, remark)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	for _, p := range rows {
		recipe := remapRecipeDepends(p.recipe, idMap)
		if _, err := tx.SQL().ExecContext(ctx, ins, p.id, workspaceID, p.name, p.kind,
			nullStr(p.repo), nullStr(p.branch), nullStr(p.work), recipe, nullStr(p.cron), p.enabled, nullStr(p.remark)); err != nil {
			return 0, err
		}
	}
	return len(rows), nil
}
