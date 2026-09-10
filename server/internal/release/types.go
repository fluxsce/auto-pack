package release

import (
	"auto-pack/server/internal/release/order"
	"auto-pack/server/internal/release/record"
)

// StartRequest is the HTTP body to start a workspace auto-release.
type StartRequest struct {
	WorkspaceRecID     string       `json:"workspace_rec_id"`
	ReleaseBranch      string       `json:"release_branch"`
	TagBranch          string       `json:"tag_branch"`
	ReleaseNote        string       `json:"release_note"`
	ReleaseDate        string       `json:"release_date"`
	AllowRepeat        *bool        `json:"allow_repeat"`
	UpdatePomVersion   *bool        `json:"update_pom_version"`
	CreateTargetBranch *bool        `json:"create_target_branch"`
	TargetBranch       string       `json:"target_branch"`
	PostOptions        *PostOptions `json:"post_options"`
}

// StartResponse is returned with HTTP 202.
type StartResponse struct {
	Status         string `json:"status"`
	ReleaseRecID   string `json:"release_rec_id"`
	WorkspaceRecID string `json:"workspace_rec_id"`
	WorkspaceName  string `json:"workspace_name,omitempty"`
	ReleaseBranch  string `json:"release_branch,omitempty"`
	Tag            string `json:"tag,omitempty"`
}

// DefaultsResponse is GET /release/defaults.
type DefaultsResponse struct {
	ReleaseBranch string `json:"release_branch"`
	TagBranch     string `json:"tag_branch"`
	TagSource     string `json:"tag_source"`
}

// SyncItem is one project in a workspace Git pre-sync.
type SyncItem struct {
	ProjectName string `json:"project_name"`
	Outcome     string `json:"outcome"`
	Action      string `json:"action,omitempty"`
	Branch      string `json:"branch,omitempty"`
	Message     string `json:"message,omitempty"`
	Log         string `json:"log,omitempty"`
}

// SyncEvent is one live line while SyncSources walks the workspace.
type SyncEvent struct {
	Event   string    `json:"event"`
	Index   int       `json:"index,omitempty"`
	Total   int       `json:"total,omitempty"`
	Item    *SyncItem `json:"item,omitempty"`
	Synced  int       `json:"synced,omitempty"`
	Skipped int       `json:"skipped,omitempty"`
	Failed  int       `json:"failed,omitempty"`
	Message string    `json:"message,omitempty"`
}

// SyncListener receives live pre-sync events. Nil is ignored.
type SyncListener func(SyncEvent)

// SyncResult is workspace Git pre-sync counts and per-project lines.
type SyncResult struct {
	Synced      int        `json:"synced"`
	Skipped     int        `json:"skipped"`
	Failed      int        `json:"failed"`
	Items       []SyncItem `json:"items,omitempty"`
	FailedItems []string   `json:"failed_items,omitempty"`
}

// Request is input for one project release.
type Request struct {
	WorkDir            string
	ReleaseBranch      string
	Tag                string
	ReleaseNote        string
	ReleaseDate        string
	AllowRepeat        bool
	UpdatePomVersion   bool
	CreateTargetBranch bool
	TargetBranch       string
}

// LogView is a tailed release log.
type LogView struct {
	Content  string `json:"content"`
	Offset   int64  `json:"offset"`
	Size     int64  `json:"size"`
	Complete bool   `json:"complete"`
	Status   string `json:"status"`
	Release  Record `json:"release"`
}

// Result is the outcome for one project.
type Result = record.Result

// Record is one AP_WORKSPACE_RELEASE row.
type Record = record.Record

// Status is the API view for poll / dialog open.
type Status = record.Status

// MavenAnalysis is the workspace Maven graph used by auto-build.
type MavenAnalysis = order.MavenAnalysis

// MavenModule is one workspace Maven project in build order.
type MavenModule = order.MavenModule

// MavenEdge is a workspace-internal Maven dependency.
type MavenEdge = order.MavenEdge

// MavenCycle is a strongly connected group that could not be ordered uniquely.
type MavenCycle = order.MavenCycle

const (
	// JobStatusIdle means no in-memory batch is tracked.
	JobStatusIdle = record.JobStatusIdle
	// JobStatusRunning is the in-memory coordinator status while a batch is active.
	JobStatusRunning = record.JobStatusRunning
	// JobStatusCompleted is the in-memory coordinator status after the batch goroutine finishes.
	JobStatusCompleted = record.JobStatusCompleted
	// RecordRunning is AP_WORKSPACE_RELEASE.status while a batch is in progress.
	RecordRunning = record.RecordRunning
	// RecordSuccess is AP_WORKSPACE_RELEASE.status when no project failed.
	RecordSuccess = record.RecordSuccess
	// RecordPartial is AP_WORKSPACE_RELEASE.status when some projects released and some failed.
	RecordPartial = record.RecordPartial
	// RecordFailed is AP_WORKSPACE_RELEASE.status when no project released and at least one failed.
	RecordFailed = record.RecordFailed
)
