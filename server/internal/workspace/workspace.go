package workspace

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/store"
)

// Workspace is a configuration boundary.
type Workspace struct {
	RecID         string `json:"rec_id"`
	WorkspaceCode string `json:"workspace_code"`
	WorkspaceName string `json:"workspace_name"`
	OwnerUserID   string `json:"owner_user_id"`
	Remark        string `json:"remark,omitempty"`
	MemberRole    string `json:"member_role,omitempty"`
	ProjectCount  int    `json:"project_count"`
	IsDefault     int    `json:"is_default"`
	RootDir       string `json:"root_dir,omitempty"`
}

// Member is a workspace member row.
type Member struct {
	RecID          string `json:"rec_id"`
	WorkspaceRecID string `json:"workspace_rec_id"`
	UserRecID      string `json:"user_rec_id"`
	LoginName      string `json:"login_name,omitempty"`
	DisplayName    string `json:"display_name,omitempty"`
	MemberRole     string `json:"member_role"`
}

var (
	// ErrLastOwner 空间必须至少保留一名所有者，否则无人能再管成员。
	ErrLastOwner = errors.New("cannot demote or remove the last owner")
	// ErrDefaultWorkspace 默认工作空间不可删除。
	ErrDefaultWorkspace = errors.New("cannot delete the default workspace")
	// ErrWorkspaceBusy 空间内仍有排队或运行中的作业。
	ErrWorkspaceBusy = errors.New("workspace has running jobs")
	// ErrWorkspaceCodeTaken 空间编码全局唯一。
	ErrWorkspaceCodeTaken = errors.New("workspace code already exists")
)

// Service manages workspaces and members.
type Service struct {
	st    *store.Store
	guard *iam.Guard
	root  string
}

// NewService returns a workspace Service. root 是所有空间与工程的检出根目录。
func NewService(st *store.Store, guard *iam.Guard, root string) *Service {
	return &Service{st: st, guard: guard, root: root}
}

// SeedDefaultWorkspace creates the default space and owner membership.
func SeedDefaultWorkspace(ctx context.Context, st *store.Store) error {
	q := st.Rebind(`SELECT COUNT(1) FROM AP_WORKSPACE`)
	var n int
	if err := st.SQL().QueryRowContext(ctx, q).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	ins := st.Rebind(`INSERT INTO AP_WORKSPACE (rec_id, workspace_code, workspace_name, owner_user_id, remark)
		VALUES (?, ?, ?, ?, ?)`)
	if _, err := st.SQL().ExecContext(ctx, ins, access.DefaultWorkspaceRecID, "default", "默认工作空间",
		access.DefaultAdminRecID, "系统初始化"); err != nil {
		return err
	}
	mem := st.Rebind(`INSERT INTO AP_WORKSPACE_MEMBER (rec_id, workspace_rec_id, user_rec_id, member_role)
		VALUES (?, ?, ?, ?)`)
	_, err := st.SQL().ExecContext(ctx, mem, access.NewID(), access.DefaultWorkspaceRecID, access.DefaultAdminRecID, iam.RoleOwner)
	return err
}

// ListForActor returns workspaces the actor belongs to.
func (s *Service) ListForActor(ctx context.Context, actor iam.Actor) ([]Workspace, error) {
	q := s.st.Rebind(`SELECT w.rec_id, w.workspace_code, w.workspace_name, w.owner_user_id, COALESCE(w.remark,''), m.member_role,
		(SELECT COUNT(1) FROM AP_PROJECT p WHERE p.workspace_rec_id = w.rec_id)
		FROM AP_WORKSPACE w
		INNER JOIN AP_WORKSPACE_MEMBER m ON m.workspace_rec_id = w.rec_id
		WHERE m.user_rec_id = ?
		ORDER BY w.workspace_name`)
	rows, err := s.st.SQL().QueryContext(ctx, q, actor.UserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.RecID, &w.WorkspaceCode, &w.WorkspaceName, &w.OwnerUserID, &w.Remark, &w.MemberRole, &w.ProjectCount); err != nil {
			return nil, err
		}
		w.IsDefault = defaultFlag(w.RecID)
		s.decorate(&w)
		out = append(out, w)
	}
	return out, rows.Err()
}

// Create makes a workspace; actor becomes owner.
func (s *Service) Create(ctx context.Context, actor iam.Actor, code, name string) (Workspace, error) {
	code = strings.TrimSpace(code)
	name = strings.TrimSpace(name)
	if code == "" || name == "" {
		return Workspace{}, errors.New("workspace_code and workspace_name required")
	}
	if err := CheckCode(code); err != nil {
		return Workspace{}, err
	}
	if err := CheckDisplayName(name); err != nil {
		return Workspace{}, err
	}
	if err := s.ensureCodeFree(ctx, code); err != nil {
		return Workspace{}, err
	}
	id := access.NewID()
	err := s.st.Tx(ctx, func(tx *store.Store) error {
		ins := tx.Rebind(`INSERT INTO AP_WORKSPACE (rec_id, workspace_code, workspace_name, owner_user_id)
			VALUES (?, ?, ?, ?)`)
		if _, err := tx.SQL().ExecContext(ctx, ins, id, code, name, actor.UserID); err != nil {
			return err
		}
		mem := tx.Rebind(`INSERT INTO AP_WORKSPACE_MEMBER (rec_id, workspace_rec_id, user_rec_id, member_role)
			VALUES (?, ?, ?, ?)`)
		_, err := tx.SQL().ExecContext(ctx, mem, access.NewID(), id, actor.UserID, iam.RoleOwner)
		return err
	})
	if err != nil {
		return Workspace{}, mapUniqueCode(err)
	}
	if err := mkdirWorkspace(s.root, code, id); err != nil {
		return Workspace{}, err
	}
	w := Workspace{RecID: id, WorkspaceCode: code, WorkspaceName: name, OwnerUserID: actor.UserID, MemberRole: iam.RoleOwner}
	s.decorate(&w)
	return w, nil
}

// ListMembers returns members (user.admin).
func (s *Service) ListMembers(ctx context.Context, actor iam.Actor, workspaceID string) ([]Member, error) {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionUserAdmin); err != nil {
		return nil, err
	}
	q := s.st.Rebind(`SELECT m.rec_id, m.workspace_rec_id, m.user_rec_id, u.login_name, u.display_name, m.member_role
		FROM AP_WORKSPACE_MEMBER m
		INNER JOIN AP_USER u ON u.rec_id = m.user_rec_id
		WHERE m.workspace_rec_id = ?
		ORDER BY m.member_role, u.login_name`)
	rows, err := s.st.SQL().QueryContext(ctx, q, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Member
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.RecID, &m.WorkspaceRecID, &m.UserRecID, &m.LoginName, &m.DisplayName, &m.MemberRole); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddMember adds or updates a member (user.admin).
func (s *Service) AddMember(ctx context.Context, actor iam.Actor, workspaceID, userID, role string) error {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionUserAdmin); err != nil {
		return err
	}
	role = strings.TrimSpace(role)
	if role != iam.RoleOwner && role != iam.RoleOperator && role != iam.RoleViewer {
		return errors.New("member_role must be owner, operator or viewer")
	}
	q := s.st.Rebind(`SELECT rec_id FROM AP_WORKSPACE_MEMBER WHERE workspace_rec_id = ? AND user_rec_id = ?`)
	var existing string
	err := s.st.SQL().QueryRowContext(ctx, q, workspaceID, userID).Scan(&existing)
	if err == nil {
		if role != iam.RoleOwner {
			if err := s.guardLastOwner(ctx, workspaceID, userID); err != nil {
				return err
			}
		}
		upd := s.st.Rebind(`UPDATE AP_WORKSPACE_MEMBER SET member_role = ? WHERE rec_id = ?`)
		_, err = s.st.SQL().ExecContext(ctx, upd, role, existing)
		return err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	ins := s.st.Rebind(`INSERT INTO AP_WORKSPACE_MEMBER (rec_id, workspace_rec_id, user_rec_id, member_role)
		VALUES (?, ?, ?, ?)`)
	_, err = s.st.SQL().ExecContext(ctx, ins, access.NewID(), workspaceID, userID, role)
	return err
}

// RemoveMember deletes a membership (user.admin).
func (s *Service) RemoveMember(ctx context.Context, actor iam.Actor, workspaceID, userID string) error {
	if err := s.guard.Allow(ctx, actor, workspaceID, iam.ActionUserAdmin); err != nil {
		return err
	}
	if err := s.guardLastOwner(ctx, workspaceID, userID); err != nil {
		return err
	}
	q := s.st.Rebind(`DELETE FROM AP_WORKSPACE_MEMBER WHERE workspace_rec_id = ? AND user_rec_id = ?`)
	_, err := s.st.SQL().ExecContext(ctx, q, workspaceID, userID)
	return err
}

// guardLastOwner 拒绝降级或移出空间内唯一的所有者。
func (s *Service) guardLastOwner(ctx context.Context, workspaceID, userID string) error {
	q := s.st.Rebind(`SELECT member_role FROM AP_WORKSPACE_MEMBER WHERE workspace_rec_id = ? AND user_rec_id = ?`)
	var role string
	err := s.st.SQL().QueryRowContext(ctx, q, workspaceID, userID).Scan(&role)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if role != iam.RoleOwner {
		return nil
	}
	cnt := s.st.Rebind(`SELECT COUNT(1) FROM AP_WORKSPACE_MEMBER WHERE workspace_rec_id = ? AND member_role = ?`)
	var n int
	if err := s.st.SQL().QueryRowContext(ctx, cnt, workspaceID, iam.RoleOwner).Scan(&n); err != nil {
		return err
	}
	if n <= 1 {
		return ErrLastOwner
	}
	return nil
}

// Update 修改空间名称与备注（所有者）。编码不可改。
func (s *Service) Update(ctx context.Context, actor iam.Actor, id, name, remark string) (Workspace, error) {
	if err := s.guard.Allow(ctx, actor, id, iam.ActionUserAdmin); err != nil {
		return Workspace{}, err
	}
	name = strings.TrimSpace(name)
	if err := CheckDisplayName(name); err != nil {
		return Workspace{}, err
	}
	q := s.st.Rebind(`UPDATE AP_WORKSPACE SET workspace_name = ?, remark = ? WHERE rec_id = ?`)
	if _, err := s.st.SQL().ExecContext(ctx, q, name, nullStr(remark), id); err != nil {
		return Workspace{}, err
	}
	return s.getForActor(ctx, actor, id)
}

// CloneRequest 是克隆空间的入参。
type CloneRequest struct {
	SourceRecID   string `json:"source_rec_id"`
	WorkspaceCode string `json:"workspace_code"`
	WorkspaceName string `json:"workspace_name"`
}

// CloneResult 是克隆结果，含已复制的工程数。
type CloneResult struct {
	Workspace
	ProjectsCloned int `json:"projects_cloned"`
}

// Clone 复制源空间的工程到新空间；调用者成为新空间所有者。不复制成员、环境与作业。
func (s *Service) Clone(ctx context.Context, actor iam.Actor, req CloneRequest) (CloneResult, error) {
	if err := s.guard.CanRead(ctx, actor, req.SourceRecID); err != nil {
		return CloneResult{}, err
	}
	code := strings.TrimSpace(req.WorkspaceCode)
	name := strings.TrimSpace(req.WorkspaceName)
	if code == "" || name == "" {
		return CloneResult{}, errors.New("workspace_code and workspace_name required")
	}
	if err := CheckCode(code); err != nil {
		return CloneResult{}, err
	}
	if err := CheckDisplayName(name); err != nil {
		return CloneResult{}, err
	}
	if err := s.ensureCodeFree(ctx, code); err != nil {
		return CloneResult{}, err
	}
	id := access.NewID()
	copied := 0
	err := s.st.Tx(ctx, func(tx *store.Store) error {
		ins := tx.Rebind(`INSERT INTO AP_WORKSPACE (rec_id, workspace_code, workspace_name, owner_user_id)
			VALUES (?, ?, ?, ?)`)
		if _, err := tx.SQL().ExecContext(ctx, ins, id, code, name, actor.UserID); err != nil {
			return err
		}
		mem := tx.Rebind(`INSERT INTO AP_WORKSPACE_MEMBER (rec_id, workspace_rec_id, user_rec_id, member_role)
			VALUES (?, ?, ?, ?)`)
		if _, err := tx.SQL().ExecContext(ctx, mem, access.NewID(), id, actor.UserID, iam.RoleOwner); err != nil {
			return err
		}
		q := tx.Rebind(`SELECT project_name, project_type, COALESCE(repo_url,''), COALESCE(repo_branch,''),
			COALESCE(work_rel_path,''), recipe_json, COALESCE(cron_expr,''), is_enabled, COALESCE(remark,'')
			FROM AP_PROJECT WHERE workspace_rec_id = ?`)
		rows, err := tx.SQL().QueryContext(ctx, q, req.SourceRecID)
		if err != nil {
			return err
		}
		defer rows.Close()
		insP := tx.Rebind(`INSERT INTO AP_PROJECT (rec_id, workspace_rec_id, project_name, project_type, repo_url, repo_branch,
			work_rel_path, recipe_json, cron_expr, is_enabled, remark)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		for rows.Next() {
			var p projectRow
			if err := rows.Scan(&p.name, &p.kind, &p.repo, &p.branch, &p.work, &p.recipe, &p.cron, &p.enabled, &p.remark); err != nil {
				return err
			}
			if _, err := tx.SQL().ExecContext(ctx, insP, access.NewID(), id, p.name, p.kind,
				nullStr(p.repo), nullStr(p.branch), nullStr(p.work), p.recipe, nullStr(p.cron), p.enabled, nullStr(p.remark)); err != nil {
				return err
			}
			copied++
		}
		return rows.Err()
	})
	if err != nil {
		return CloneResult{}, mapUniqueCode(err)
	}
	if err := mkdirWorkspace(s.root, code, id); err != nil {
		return CloneResult{}, err
	}
	ws, err := s.getForActor(ctx, actor, id)
	if err != nil {
		return CloneResult{}, err
	}
	return CloneResult{Workspace: ws, ProjectsCloned: copied}, nil
}

// Delete 删除空间及其工程、环境、作业与成员。默认空间不可删；有运行中作业时拒绝。
func (s *Service) Delete(ctx context.Context, actor iam.Actor, id string) error {
	if id == access.DefaultWorkspaceRecID {
		return ErrDefaultWorkspace
	}
	if err := s.guard.Allow(ctx, actor, id, iam.ActionUserAdmin); err != nil {
		return err
	}
	busy := s.st.Rebind(`SELECT COUNT(1) FROM AP_JOB WHERE workspace_rec_id = ? AND job_status IN ('queued','running')`)
	var n int
	if err := s.st.SQL().QueryRowContext(ctx, busy, id).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrWorkspaceBusy
	}
	codeQ := s.st.Rebind(`SELECT workspace_code FROM AP_WORKSPACE WHERE rec_id = ?`)
	var code string
	if err := s.st.SQL().QueryRowContext(ctx, codeQ, id).Scan(&code); err != nil {
		return err
	}
	err := s.st.Tx(ctx, func(tx *store.Store) error {
		steps := tx.Rebind(`DELETE FROM AP_JOB_STEP WHERE job_rec_id IN (SELECT rec_id FROM AP_JOB WHERE workspace_rec_id = ?)`)
		if _, err := tx.SQL().ExecContext(ctx, steps, id); err != nil {
			return err
		}
		jobs := tx.Rebind(`DELETE FROM AP_JOB WHERE workspace_rec_id = ?`)
		if _, err := tx.SQL().ExecContext(ctx, jobs, id); err != nil {
			return err
		}
		prjs := tx.Rebind(`DELETE FROM AP_PROJECT WHERE workspace_rec_id = ?`)
		if _, err := tx.SQL().ExecContext(ctx, prjs, id); err != nil {
			return err
		}
		envs := tx.Rebind(`DELETE FROM AP_ENV WHERE workspace_rec_id = ?`)
		if _, err := tx.SQL().ExecContext(ctx, envs, id); err != nil {
			return err
		}
		mems := tx.Rebind(`DELETE FROM AP_WORKSPACE_MEMBER WHERE workspace_rec_id = ?`)
		if _, err := tx.SQL().ExecContext(ctx, mems, id); err != nil {
			return err
		}
		ws := tx.Rebind(`DELETE FROM AP_WORKSPACE WHERE rec_id = ?`)
		_, err := tx.SQL().ExecContext(ctx, ws, id)
		return err
	})
	if err == nil {
		removeWorkspaceDir(s.root, code, id)
	}
	return err
}

type projectRow struct {
	name, kind, repo, branch, work, recipe, cron, remark string
	enabled                                              int
}

// GetRaw loads a workspace without IAM. Callers must authorize first.
func (s *Service) GetRaw(ctx context.Context, id string) (Workspace, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Workspace{}, errors.New("workspace not found")
	}
	q := s.st.Rebind(`SELECT rec_id, workspace_code, workspace_name, owner_user_id, COALESCE(remark,'') FROM AP_WORKSPACE WHERE rec_id=?`)
	var w Workspace
	err := s.st.SQL().QueryRowContext(ctx, q, id).Scan(&w.RecID, &w.WorkspaceCode, &w.WorkspaceName, &w.OwnerUserID, &w.Remark)
	if errors.Is(err, sql.ErrNoRows) {
		return Workspace{}, errors.New("workspace not found")
	}
	if err != nil {
		return Workspace{}, err
	}
	s.decorate(&w)
	return w, nil
}

func (s *Service) getForActor(ctx context.Context, actor iam.Actor, id string) (Workspace, error) {
	list, err := s.ListForActor(ctx, actor)
	if err != nil {
		return Workspace{}, err
	}
	for _, w := range list {
		if w.RecID == id {
			return w, nil
		}
	}
	return Workspace{}, errors.New("workspace not found")
}

// lookupIDByCode 按编码查空间主键；不存在时 id 为空。
func (s *Service) lookupIDByCode(ctx context.Context, code string) (string, error) {
	q := s.st.Rebind(`SELECT rec_id FROM AP_WORKSPACE WHERE workspace_code = ?`)
	var id string
	err := s.st.SQL().QueryRowContext(ctx, q, code).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return id, err
}

// ensureCodeFree 在插入前检查编码是否已被占用。
func (s *Service) ensureCodeFree(ctx context.Context, code string) error {
	id, err := s.lookupIDByCode(ctx, code)
	if err != nil {
		return err
	}
	if id != "" {
		return ErrWorkspaceCodeTaken
	}
	return nil
}

func mapUniqueCode(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrWorkspaceCodeTaken) {
		return err
	}
	if isUniqueViolation(err) {
		return ErrWorkspaceCodeTaken
	}
	return err
}

// isUniqueViolation 识别 MySQL 1062 / SQLite UNIQUE / Postgres duplicate key。
func isUniqueViolation(err error) bool {
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate")
}

func (s *Service) decorate(w *Workspace) {
	w.RootDir = Dir(s.root, w.WorkspaceCode, w.RecID)
}

func defaultFlag(id string) int {
	if id == access.DefaultWorkspaceRecID {
		return 1
	}
	return 0
}

func nullStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
