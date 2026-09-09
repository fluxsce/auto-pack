package project

import (
	"net/http"
	"strings"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/projectrelease"
)

type batchAutoReleaseDefaultsResponse struct {
	ReleaseBranch string                      `json:"release_branch"`
	TagBranch     string                      `json:"tag_branch"`
	TagSource     string                      `json:"tag_source,omitempty"` // remote | history | workspace
	PostOptions   *projectrelease.PostOptions `json:"post_options,omitempty"`
}

func (h *Handler) batchAutoReleaseDefaults(w http.ResponseWriter, r *http.Request) {
	wsID := strings.TrimSpace(r.URL.Query().Get("workspace_rec_id"))
	if wsID == "" {
		httpx.WriteError(w, http.StatusBadRequest, "workspace_rec_id is required")
		return
	}
	ws, projects, err := h.loadWorkspaceProjects(wsID)
	if err != nil {
		writeWorkspaceLoadError(w, err)
		return
	}
	code := strings.TrimSpace(ws.WorkspaceCode)
	releaseBranch := projectrelease.DefaultReleaseBranch(code)
	tagBranch := code
	tagSource := "workspace"

	ctx := r.Context()
	committer := &projectrelease.GitCommitter{EnvRepo: h.env}
	if remoteTags, ok := projectrelease.CollectWorkspaceRemoteTags(ctx, committer, ws, projects, h.dataDir); ok && len(remoteTags) > 0 {
		tagBranch = projectrelease.SuggestNextVersionTag(code, remoteTags)
		tagSource = "remote"
	} else if histTags := h.collectReleaseHistoryTags(ws.RecID); len(histTags) > 0 {
		tagBranch = projectrelease.SuggestNextVersionTag(code, histTags)
		tagSource = "history"
	}

	httpx.WriteJSON(w, http.StatusOK, batchAutoReleaseDefaultsResponse{
		ReleaseBranch: releaseBranch,
		TagBranch:     tagBranch,
		TagSource:     tagSource,
	})
}

func (h *Handler) collectReleaseHistoryTags(workspaceRecID string) []string {
	items, err := h.releases.ListByWorkspace(workspaceRecID, 50)
	if err != nil {
		return nil
	}
	seen := make(map[string]struct{})
	var tags []string
	for _, item := range items {
		t := projectrelease.NormalizeVersionTag(item.TagBranch)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		tags = append(tags, t)
	}
	return tags
}
