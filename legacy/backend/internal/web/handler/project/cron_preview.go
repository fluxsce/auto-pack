package project

import (
	"encoding/json"
	"net/http"
	"time"

	"auto-pack/backend/internal/httpx"
	"auto-pack/backend/pkg/projectcfg"
)

type cronPreviewPayload struct {
	Cron  string `json:"cron"`
	Count int    `json:"count"`
}

func (h *Handler) previewCron(w http.ResponseWriter, r *http.Request) {
	var body cronPreviewPayload
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	runs, err := projectcfg.PreviewCronSchedule(body.Cron, body.Count)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	zoneName, _ := time.Now().Zone()
	formatted := make([]string, len(runs))
	for i, t := range runs {
		formatted[i] = t.Format("2006-01-02 15:04:05")
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"next_runs": formatted,
		"timezone":  zoneName,
	})
}
