package env

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/audit"
	"auto-pack/backend/pkg/envcfg"
	"auto-pack/backend/pkg/envverify"

	"github.com/google/uuid"
)

// Handler manages /api/env/settings CRUD and verify.
type Handler struct {
	repo  *repository.EnvRepository
	audit *audit.Recorder
}

func NewHandler(repo *repository.EnvRepository, rec *audit.Recorder) *Handler {
	return &Handler{repo: repo, audit: rec}
}

// Register mounts env settings routes (requires auth wrapper).
func Register(mux *http.ServeMux, h *Handler, requireAuth func(http.Handler) http.Handler, apiBase string) {
	base := apiBase + "/env/settings"
	mux.Handle("GET "+base, requireAuth(http.HandlerFunc(h.list)))
	mux.Handle("POST "+base, requireAuth(http.HandlerFunc(h.create)))
	mux.Handle("POST "+base+"/probe", requireAuth(http.HandlerFunc(h.probe)))
	mux.Handle("GET "+base+"/{recID}", requireAuth(http.HandlerFunc(h.get)))
	mux.Handle("PUT "+base+"/{recID}", requireAuth(http.HandlerFunc(h.update)))
	mux.Handle("DELETE "+base+"/{recID}", requireAuth(http.HandlerFunc(h.delete)))
	mux.Handle("POST "+base+"/{recID}/verify", requireAuth(http.HandlerFunc(h.verify)))
	mux.Handle("GET "+apiBase+"/env/types", requireAuth(http.HandlerFunc(h.types)))
}

func (h *Handler) types(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"types": envcfg.AllTypes})
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	envType := strings.TrimSpace(r.URL.Query().Get("env_type"))
	items, err := h.repo.List(envType)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]repository.EnvSetting, 0, len(items))
	for _, item := range items {
		item.ConfigJSON = envcfg.MaskConfigJSON(item.ConfigJSON)
		out = append(out, item)
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"items": out})
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	item, err := h.repo.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	item.ConfigJSON = envcfg.MaskConfigJSON(item.ConfigJSON)
	httpx.WriteJSON(w, http.StatusOK, item)
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var body envPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := body.toModel("")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	item.RecID = uuid.New().String()
	if item.IsDefault == 1 {
		_ = h.repo.ClearDefaultByType(item.EnvType, item.RecID)
	}
	if err := h.repo.Insert(item); err != nil {
		writeRepoError(w, err)
		return
	}
	created, _ := h.repo.FindByRecID(item.RecID)
	created.ConfigJSON = envcfg.MaskConfigJSON(created.ConfigJSON)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionCreate, Module: audit.ModuleEnv, TargetType: "env",
		TargetID: created.RecID, TargetName: created.EnvName,
		Summary: "创建环境 " + created.EnvName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusCreated, created)
}

func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	existing, err := h.repo.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	var body envPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	item, err := body.toModel(recID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	merged, err := envcfg.MergeSecrets(existing.ConfigJSON, item.ConfigJSON)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	item.ConfigJSON = merged
	if item.IsDefault == 1 {
		_ = h.repo.ClearDefaultByType(item.EnvType, item.RecID)
	}
	if err := h.repo.Update(item); err != nil {
		writeRepoError(w, err)
		return
	}
	updated, _ := h.repo.FindByRecID(recID)
	updated.ConfigJSON = envcfg.MaskConfigJSON(updated.ConfigJSON)
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionUpdate, Module: audit.ModuleEnv, TargetType: "env",
		TargetID: updated.RecID, TargetName: updated.EnvName,
		Summary: "更新环境 " + updated.EnvName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, updated)
}

func (h *Handler) delete(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	existing, err := h.repo.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if err := h.repo.Delete(recID); err != nil {
		writeRepoError(w, err)
		return
	}
	h.audit.Record(r.Context(), r, audit.Entry{
		Action: audit.ActionDelete, Module: audit.ModuleEnv, TargetType: "env",
		TargetID: recID, TargetName: existing.EnvName,
		Summary: "删除环境 " + existing.EnvName, Success: true,
	})
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (h *Handler) probe(w http.ResponseWriter, r *http.Request) {
	var body probePayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	cfgJSON, envType, err := body.resolveConfig(h)
	if err != nil {
		if errors.Is(err, repository.ErrEnvNotFound) {
			writeRepoError(w, err)
			return
		}
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := envverify.Run(r.Context(), envType, cfgJSON)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) {
	recID := r.PathValue("recID")
	item, err := h.repo.FindByRecID(recID)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	if item.IsEnabled != 1 {
		httpx.WriteError(w, http.StatusBadRequest, "env setting is disabled")
		return
	}
	result, err := envverify.Run(r.Context(), item.EnvType, item.ConfigJSON)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, result)
}

type probePayload struct {
	EnvType        string `json:"env_type"`
	ConfigJSON     string `json:"config_json"`
	MergeFromRecID string `json:"merge_from_rec_id,omitempty"`
}

func (p *probePayload) resolveConfig(h *Handler) (cfgJSON, envType string, err error) {
	envType = strings.TrimSpace(strings.ToLower(p.EnvType))
	if envType == "" {
		return "", "", errors.New("env_type is required")
	}
	if !envcfg.IsValidType(envType) {
		return "", "", errors.New("invalid env_type")
	}
	cfgJSON = strings.TrimSpace(p.ConfigJSON)
	if cfgJSON == "" {
		cfgJSON = "{}"
	}
	if mergeID := strings.TrimSpace(p.MergeFromRecID); mergeID != "" {
		existing, findErr := h.repo.FindByRecID(mergeID)
		if findErr != nil {
			return "", "", findErr
		}
		cfgJSON, err = envcfg.MergeSecrets(existing.ConfigJSON, cfgJSON)
		if err != nil {
			return "", "", err
		}
	}
	if _, err = envcfg.Parse(envType, cfgJSON); err != nil {
		return "", "", err
	}
	return cfgJSON, envType, nil
}

type envPayload struct {
	EnvName    string `json:"env_name"`
	EnvType    string `json:"env_type"`
	ConfigJSON string `json:"config_json"`
	IsDefault  int    `json:"is_default"`
	IsEnabled  int    `json:"is_enabled"`
	SortNum    int    `json:"sort_num"`
	Remark     string `json:"remark"`
}

func (p *envPayload) toModel(recID string) (*repository.EnvSetting, error) {
	name := strings.TrimSpace(p.EnvName)
	envType := strings.TrimSpace(strings.ToLower(p.EnvType))
	if name == "" {
		return nil, errors.New("env_name is required")
	}
	if !envcfg.IsValidType(envType) {
		return nil, errors.New("invalid env_type")
	}
	cfg := strings.TrimSpace(p.ConfigJSON)
	if cfg == "" {
		cfg = "{}"
	}
	if _, err := envcfg.Parse(envType, cfg); err != nil {
		return nil, err
	}
	isDefault := 0
	if p.IsDefault == 1 {
		isDefault = 1
	}
	isEnabled := 1
	if p.IsEnabled == 0 {
		isEnabled = 0
	}
	return &repository.EnvSetting{
		RecID:      recID,
		EnvName:    name,
		EnvType:    envType,
		ConfigJSON: cfg,
		IsDefault:  isDefault,
		IsEnabled:  isEnabled,
		SortNum:    p.SortNum,
		Remark:     strings.TrimSpace(p.Remark),
	}, nil
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrEnvNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, repository.ErrEnvNameExists):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	default:
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
	}
}
