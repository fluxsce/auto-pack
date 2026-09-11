package build

import "auto-pack/server/internal/build/record"

// StartRequest is the HTTP body to start a workspace auto-build.
type StartRequest struct {
	WorkspaceRecID  string   `json:"workspace_rec_id"`
	Scope           string   `json:"scope"`
	ProjectRecIDs   []string `json:"project_rec_ids"`
	IncludeNonMaven bool     `json:"include_non_maven"`
	// ExactTargets 为 true 时只构建 project_rec_ids，不再按 pom.xml 展开全部上游。
	ExactTargets bool `json:"exact_targets"`
}

// StartResponse is returned with HTTP 202.
type StartResponse struct {
	Status         string `json:"status"`
	BuildRecID     string `json:"build_rec_id"`
	WorkspaceRecID string `json:"workspace_rec_id"`
	WorkspaceName  string `json:"workspace_name,omitempty"`
	TotalCount     int    `json:"total_count"`
}

// LogView is a tailed build log.
type LogView struct {
	Content  string `json:"content"`
	Offset   int64  `json:"offset"`
	Size     int64  `json:"size"`
	Complete bool   `json:"complete"`
	Status   string `json:"status"`
	Build    Record `json:"build"`
}

// Record is one AP_WORKSPACE_BUILD row.
type Record = record.Record

// Status is the API view for poll / dialog open.
type Status = record.Status

const (
	// JobStatusIdle means no in-memory batch is tracked.
	JobStatusIdle = record.JobStatusIdle
	// JobStatusRunning is the in-memory coordinator status while a batch is active.
	JobStatusRunning = record.JobStatusRunning
	// JobStatusCompleted is the in-memory coordinator status after the batch goroutine finishes.
	JobStatusCompleted = record.JobStatusCompleted
	// RecordRunning is AP_WORKSPACE_BUILD.status while a batch is in progress.
	RecordRunning = record.RecordRunning
)
