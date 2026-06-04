package user

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/internal/web/auth"
	"auto-pack/backend/internal/web/middleware"
	"auto-pack/backend/pkg/audit"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Handler manages /api/users admin CRUD.
type Handler struct {
	users *repository.UserRepository
	audit *audit.Recorder
}

func NewHandler(users *repository.UserRepository, rec *audit.Recorder) *Handler {
	return &Handler{users: users, audit: rec}
}

// Register mounts user admin routes.
func Register(mux *http.ServeMux, h *Handler, requireAuth func(http.Handler) http.Handler, apiBase string) {
	base := apiBase + "/users"
	mux.Handle("GET "+base, requireAuth(http.HandlerFunc(h.list)))
	mux.Handle("POST "+base, requireAuth(http.HandlerFunc(h.create)))
	mux.Handle("GET "+base+"/{recID}", requireAuth(http.HandlerFunc(h.get)))
	mux.Handle("PUT "+base+"/{recID}", requireAuth(http.HandlerFunc(h.update)))
	mux.Handle("DELETE "+base+"/{recID}", requireAuth(http.HandlerFunc(h.delete)))
	mux.Handle("POST "+base+"/{recID}/reset-password", requireAuth(http.HandlerFunc(h.resetPassword)))
}

type userPayload struct {
	LoginName string `json:"login_name"`
	Password  string `json:"password"`
	IsEnabled int    `json:"is_enabled"`
	Remark    string `json:"remark"`
}

type resetPasswordPayload struct {
	NewPassword string `json:"new_password"`
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	items, err := h.users.List()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	item, err := h.users.FindByRecID(r.PathValue("recID"))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body userPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	loginName := strings.TrimSpace(body.LoginName)
	if loginName == "" {
		httpx.WriteError(w, http.StatusBadRequest, "login_name is required")
		return
	}
	if len(strings.TrimSpace(body.Password)) < 6 {
		httpx.WriteError(w, http.StatusBadRequest, auth.ErrWeakPassword.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	isEnabled := 1
	if body.IsEnabled == 0 {
		isEnabled = 0
	}
	item := &repository.User{
		RecID:        uuid.New().String(),
		LoginName:    loginName,
		PasswordHash: string(hash),
		IsEnabled:    isEnabled,
		Remark:       strings.TrimSpace(body.Remark),
	}
	if err := h.users.Insert(item); err != nil {
		writeRepoError(w, err)
		h.audit.Record(r.Context(), r, audit.Entry{
			Action: audit.ActionCreate, Module: audit.ModuleUser, TargetType: "user",
			TargetName: loginName, Summary: "创建用户失败", Success: false, ErrorMessage: err.Error(),
		})
		return
	}
	created, _ := h.users.FindByRecID(item.RecID)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionCreate, Module: audit.ModuleUser, TargetType: "user",
		TargetID: created.RecID, TargetName: created.LoginName,
		Summary: "创建用户 " + created.LoginName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	existing, err := h.users.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	var body userPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	loginName := strings.TrimSpace(body.LoginName)
	if loginName == "" {
		httpx.WriteError(w, http.StatusBadRequest, "login_name is required")
		return
	}
	isEnabled := 1
	if body.IsEnabled == 0 {
		isEnabled = 0
	}
	if isEnabled == 0 {
		if err := h.ensureNotLastEnabled(recID, existing.IsEnabled); err != nil {
			writeRepoError(w, err)
			return
		}
	}
	if err := h.users.UpdateProfile(recID, loginName, isEnabled, strings.TrimSpace(body.Remark)); err != nil {
		writeRepoError(w, err)
		return
	}
	updated, _ := h.users.FindByRecID(recID)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleUser, TargetType: "user",
		TargetID: updated.RecID, TargetName: updated.LoginName,
		Summary: "更新用户 " + updated.LoginName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, updated)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	existing, err := h.users.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if actor, ok := middleware.RecIDFromContext(r.Context()); ok && actor == recID {
		httpx.WriteError(w, http.StatusBadRequest, repository.ErrUserCannotDeleteSelf.Error())
		return
	}
	if err := h.ensureNotLastEnabled(recID, existing.IsEnabled); err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.users.Delete(recID); err != nil {
		writeRepoError(w, err)
		return
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionDelete, Module: audit.ModuleUser, TargetType: "user",
		TargetID: recID, TargetName: existing.LoginName,
		Summary: "删除用户 " + existing.LoginName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	existing, err := h.users.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	var body resetPasswordPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len(strings.TrimSpace(body.NewPassword)) < 6 {
		httpx.WriteError(w, http.StatusBadRequest, auth.ErrWeakPassword.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(body.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := h.users.UpdatePassword(recID, string(hash)); err != nil {
		writeRepoError(w, err)
		return
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionResetPassword, Module: audit.ModuleUser, TargetType: "user",
		TargetID: recID, TargetName: existing.LoginName,
		Summary: "重置用户密码 " + existing.LoginName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "ok"})
}

func (h *Handler) ensureNotLastEnabled(recID string, currentlyEnabled int) error {
	if currentlyEnabled != 1 {
		return nil
	}
	n, err := h.users.CountEnabled()
	if err != nil {
		return err
	}
	if n <= 1 {
		return repository.ErrUserLastEnabled
	}
	_ = recID
	return nil
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrUserLoginNameExists):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, repository.ErrUserCannotDeleteSelf):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, repository.ErrUserLastEnabled):
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
