package audit

import (
	"context"
	"fmt"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/store"
)

const (
	// Write action codes stored in AP_AUDIT_LOG.action_code.
	ActionJobSubmit       = "job.submit"
	ActionJobCancel       = "job.cancel"
	ActionProjectSave     = "project.save"
	ActionProjectClone    = "project.clone"
	ActionProjectDelete   = "project.delete"
	ActionProjectBranch   = "project.branch"
	ActionProjectPost       = "project.post"
	ActionProjectCommand    = "project.command"
	ActionProjectClearFiles = "project.clear_files"
	ActionProjectDeleteFile = "project.delete_file"
	ActionEnvSave         = "env.save"
	ActionEnvDelete       = "env.delete"
	ActionEnvMail         = "env.mail"
	ActionUserCreate      = "user.create"
	ActionRoleReplace     = "role.replace"
	ActionWorkspaceCreate = "workspace.create"
	ActionWorkspaceSave   = "workspace.save"
	ActionWorkspaceClone  = "workspace.clone"
	ActionWorkspaceExport = "workspace.export"
	ActionWorkspaceImport = "workspace.import"
	ActionWorkspaceDelete = "workspace.delete"
	ActionMemberAdd       = "member.add"
	ActionMemberRemove    = "member.remove"
	ActionSettingsSave    = "settings.save"
	ActionVarSave         = "settings.var.save"
	ActionVarDelete       = "settings.var.delete"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	maxSummaryRunes = 400
)

// Entry is one audit row.
type Entry struct {
	RecID            string `json:"rec_id"`
	ActorUserID      string `json:"actor_user_id"`
	ActorLoginName   string `json:"actor_login_name,omitempty"`
	ActorDisplayName string `json:"actor_display_name,omitempty"`
	WorkspaceRecID   string `json:"workspace_rec_id,omitempty"`
	ActionCode       string `json:"action_code"`
	TargetType       string `json:"target_type,omitempty"`
	TargetID         string `json:"target_id,omitempty"`
	Summary          string `json:"summary"`
	SuccessFlag      int    `json:"success_flag"`
	CreatedAt        string `json:"created_at,omitempty"`
}

// ListQuery filters and pages workspace audit rows.
type ListQuery struct {
	WorkspaceID string
	Action      string
	Result      string
	Keyword     string
	Page        int
	PageSize    int
}

// Page is a paged audit list.
type Page struct {
	Items    []Entry `json:"items"`
	Total    int     `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

// Service writes and lists audit logs.
type Service struct {
	st    *store.Store
	guard *iam.Guard
}

// NewService returns an audit Service.
func NewService(st *store.Store, guard *iam.Guard) *Service {
	return &Service{st: st, guard: guard}
}

// Record inserts an audit row (no IAM; callers already passed the gate).
func (s *Service) Record(ctx context.Context, actor iam.Actor, workspaceID, action, targetType, targetID, summary string, ok bool) error {
	if s == nil || s.st == nil {
		return fmt.Errorf("audit: service not configured")
	}
	action = strings.TrimSpace(action)
	if action == "" {
		return fmt.Errorf("audit: action is required")
	}
	flag := 1
	if !ok {
		flag = 0
	}
	q := s.st.Rebind(`INSERT INTO AP_AUDIT_LOG (rec_id, actor_user_id, workspace_rec_id, action_code, target_type, target_id, summary, success_flag)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	var ws any
	if workspaceID != "" {
		ws = workspaceID
	}
	_, err := s.st.SQL().ExecContext(ctx, q, access.NewID(), actor.UserID, ws, action, null(targetType), null(targetID), clip(summary, maxSummaryRunes), flag)
	return err
}

// Note records a write attempt. Insert errors are ignored so they never fail the caller.
func (s *Service) Note(ctx context.Context, actor iam.Actor, workspaceID, action, targetType, targetID, summary string, opErr error) {
	if s == nil {
		return
	}
	if opErr != nil {
		msg := strings.TrimSpace(opErr.Error())
		if msg != "" {
			if summary == "" {
				summary = msg
			} else {
				summary = summary + ": " + msg
			}
		}
	}
	_ = s.Record(ctx, actor, workspaceID, action, targetType, targetID, summary, opErr == nil)
}

// List returns recent audit rows in one workspace (audit.read).
func (s *Service) List(ctx context.Context, actor iam.Actor, in ListQuery) (Page, error) {
	if err := s.guard.Allow(ctx, actor, in.WorkspaceID, iam.ActionAuditRead); err != nil {
		return Page{}, err
	}
	page, size := normalizePage(in.Page, in.PageSize)
	where, args := listWhere(in)
	cond := strings.Join(where, " AND ")

	countQ := s.st.Rebind(`SELECT COUNT(1) FROM AP_AUDIT_LOG a
		LEFT JOIN AP_USER u ON u.rec_id = a.actor_user_id
		WHERE ` + cond)
	var total int
	if err := s.st.SQL().QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return Page{}, err
	}

	listQ := s.st.Rebind(`SELECT a.rec_id, a.actor_user_id, COALESCE(u.login_name,''), COALESCE(u.display_name,''),
		COALESCE(a.workspace_rec_id,''), a.action_code, COALESCE(a.target_type,''), COALESCE(a.target_id,''),
		a.summary, a.success_flag, a.created_at
		FROM AP_AUDIT_LOG a
		LEFT JOIN AP_USER u ON u.rec_id = a.actor_user_id
		WHERE ` + cond + `
		ORDER BY a.created_at DESC LIMIT ? OFFSET ?`)
	listArgs := append(append([]any{}, args...), size, (page-1)*size)
	rows, err := s.st.SQL().QueryContext(ctx, listQ, listArgs...)
	if err != nil {
		return Page{}, err
	}
	defer rows.Close()
	out := make([]Entry, 0)
	for rows.Next() {
		var e Entry
		if err := rows.Scan(&e.RecID, &e.ActorUserID, &e.ActorLoginName, &e.ActorDisplayName, &e.WorkspaceRecID,
			&e.ActionCode, &e.TargetType, &e.TargetID, &e.Summary, &e.SuccessFlag, &e.CreatedAt); err != nil {
			return Page{}, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	return Page{Items: out, Total: total, Page: page, PageSize: size}, nil
}

func listWhere(in ListQuery) ([]string, []any) {
	where := []string{"a.workspace_rec_id = ?"}
	args := []any{strings.TrimSpace(in.WorkspaceID)}
	if action := strings.TrimSpace(in.Action); action != "" {
		where = append(where, "a.action_code = ?")
		args = append(args, action)
	}
	switch strings.ToLower(strings.TrimSpace(in.Result)) {
	case "ok":
		where = append(where, "a.success_flag = ?")
		args = append(args, 1)
	case "fail":
		where = append(where, "a.success_flag = ?")
		args = append(args, 0)
	}
	if kw := strings.TrimSpace(in.Keyword); kw != "" {
		like := "%" + kw + "%"
		where = append(where, "(a.summary LIKE ? OR a.action_code LIKE ? OR COALESCE(u.login_name,'') LIKE ? OR COALESCE(u.display_name,'') LIKE ?)")
		args = append(args, like, like, like, like)
	}
	return where, args
}

func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = defaultPageSize
	}
	if size > maxPageSize {
		size = maxPageSize
	}
	return page, size
}

func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func clip(s string, n int) string {
	r := []rune(strings.TrimSpace(s))
	if n <= 0 || len(r) <= n {
		return string(r)
	}
	return string(r[:n])
}
