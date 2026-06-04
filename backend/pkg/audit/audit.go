package audit

import (
	"context"
	"net"
	"net/http"
	"strings"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/internal/web/middleware"

	"github.com/google/uuid"
)

// Action names.
const (
	ActionCreate        = "create"
	ActionUpdate        = "update"
	ActionDelete        = "delete"
	ActionClone         = "clone"
	ActionLogin         = "login"
	ActionLogout        = "logout"
	ActionResetPassword = "reset_password"
	ActionBuild         = "build"
	ActionVerify        = "verify"
)

// Module names.
const (
	ModuleAuth      = "auth"
	ModuleUser      = "user"
	ModuleEnv       = "env"
	ModuleProject   = "project"
	ModuleWorkspace = "workspace"
	ModuleBuild     = "build"
)

// Recorder persists operation audit entries.
type Recorder struct {
	repo  *repository.OperationLogRepository
	users *repository.UserRepository
}

func NewRecorder(repo *repository.OperationLogRepository, users *repository.UserRepository) *Recorder {
	return &Recorder{repo: repo, users: users}
}

// Entry describes one audit event.
type Entry struct {
	Action       string
	Module       string
	TargetType   string
	TargetID     string
	TargetName   string
	Summary      string
	Detail       string
	Success      bool
	ErrorMessage string
	UserRecID    string
	LoginName    string
}

// Record stores an audit log row (best-effort; errors are ignored).
func (r *Recorder) Record(ctx context.Context, req *http.Request, e Entry) {
	if r == nil || r.repo == nil {
		return
	}
	userID := strings.TrimSpace(e.UserRecID)
	loginName := strings.TrimSpace(e.LoginName)
	if userID == "" {
		if id, ok := middleware.RecIDFromContext(ctx); ok {
			userID = id
		}
	}
	if loginName == "" && userID != "" && r.users != nil {
		if u, err := r.users.FindByRecID(userID); err == nil {
			loginName = u.LoginName
		}
	}
	success := 0
	if e.Success {
		success = 1
	}
	method, path, ip := "", "", ""
	if req != nil {
		method = req.Method
		path = req.URL.Path
		ip = ClientIP(req)
	}
	_ = r.repo.Insert(&repository.OperationLog{
		RecID:         uuid.New().String(),
		UserRecID:     userID,
		LoginName:     loginName,
		Action:        strings.TrimSpace(e.Action),
		Module:        strings.TrimSpace(e.Module),
		TargetType:    strings.TrimSpace(e.TargetType),
		TargetID:      strings.TrimSpace(e.TargetID),
		TargetName:    strings.TrimSpace(e.TargetName),
		Summary:       strings.TrimSpace(e.Summary),
		Detail:        strings.TrimSpace(e.Detail),
		RequestMethod: method,
		RequestPath:   path,
		ClientIP:      ip,
		Success:       success,
		ErrorMessage:  strings.TrimSpace(e.ErrorMessage),
	})
}

// ClientIP resolves the client address from the request.
func ClientIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}
