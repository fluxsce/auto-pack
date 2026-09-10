// Package job 是作业控制面：入队、查询、取消与 cron。执行面在 job/runner。
package job

import (
	"encoding/json"
	"errors"
	"time"
)

var (
	// ErrPOMVersion is returned when a pom_version job has no version.
	ErrPOMVersion = errors.New("pom_version required")
	// ErrUnknownBatchType is returned when batch submit gets an unsupported job_type.
	ErrUnknownBatchType = errors.New("unsupported batch job_type")
)

const (
	TypeBuild       = "build"
	TypeRelease     = "release"
	TypeFTPPush     = "ftp_push"
	TypePOMVersion  = "pom_version"
	TypeBranchClone = "branch_clone"
	TypeSyncSources = "sync_sources"
)

const (
	// OriginAutoBuild 工作空间自动构建提交的工程作业。
	OriginAutoBuild = "auto_build"
	// OriginAutoRelease 发版后自动构建提交的工程作业。
	OriginAutoRelease = "auto_release"
)

const (
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusSucceeded = "succeeded"
	StatusFailed    = "failed"
	StatusCanceled  = "canceled"
)

const (
	StepPending   = "pending"
	StepRunning   = "running"
	StepSucceeded = "succeeded"
	StepFailed    = "failed"
	StepSkipped   = "skipped"
	StepCanceled  = "canceled"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
	// TimeLayout is the UTC timestamp stored on AP_JOB time columns.
	TimeLayout = "2006-01-02 15:04:05"
)

// ListQuery filters and pages workspace jobs.
type ListQuery struct {
	WorkspaceID string
	ProjectID   string
	JobType     string
	Page        int
	PageSize    int
}

// Stats counts jobs in the current list filter.
type Stats struct {
	Running   int `json:"running"`
	Queued    int `json:"queued"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	Canceled  int `json:"canceled"`
}

// Page is a paged job list plus live rows and filter-wide counts.
type Page struct {
	Items    []Job `json:"items"`
	Live     []Job `json:"live"`
	Stats    Stats `json:"stats"`
	Total    int   `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
}

// Job is one AP_JOB row.
type Job struct {
	RecID          string `json:"rec_id"`
	WorkspaceRecID string `json:"workspace_rec_id"`
	ProjectRecID   string `json:"project_rec_id,omitempty"`
	JobType        string `json:"job_type"`
	JobStatus      string `json:"job_status"`
	PayloadJSON    string `json:"payload_json"`
	ErrorText      string `json:"error_text,omitempty"`
	CreatedBy      string `json:"created_by"`
	StartedAt      string `json:"started_at,omitempty"`
	FinishedAt     string `json:"finished_at,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
}

// Step is one AP_JOB_STEP row.
type Step struct {
	RecID      string `json:"rec_id"`
	JobRecID   string `json:"job_rec_id"`
	StepKey    string `json:"step_key"`
	StepName   string `json:"step_name"`
	SortNum    int    `json:"sort_num"`
	StepStatus string `json:"step_status"`
	ErrorText  string `json:"error_text,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

// Payload is the JSON stored on AP_JOB.payload_json.
type Payload struct {
	Tag         string   `json:"tag,omitempty"`
	Branch      string   `json:"branch,omitempty"`
	FromBranch  string   `json:"from_branch,omitempty"`
	Force       *bool    `json:"force,omitempty"`
	Notes       string   `json:"notes,omitempty"`
	UpdatePOM   bool     `json:"update_pom,omitempty"`
	POMVersion  string   `json:"pom_version,omitempty"`
	RemotePath  string   `json:"remote_path,omitempty"`
	BackupPath  string   `json:"backup_path,omitempty"`
	Pusher      string   `json:"pusher,omitempty"`
	TaskRef     string   `json:"task_ref,omitempty"`
	Detail      string   `json:"detail,omitempty"`
	Artifact    string   `json:"artifact,omitempty"`
	Artifacts   []string `json:"artifacts,omitempty"`
	Command     string   `json:"command,omitempty"`
	EnvRecID    string   `json:"env_rec_id,omitempty"`
	CloneURL    string   `json:"clone_url,omitempty"`
	CloneDir    string   `json:"clone_dir,omitempty"`
	DependChain []string `json:"depend_chain,omitempty"`
	// SkipSync 批次已统一预同步时跳过作业内再次 fetch/checkout。
	SkipSync bool `json:"skip_sync,omitempty"`
	// Origin 标记作业来源，供工程构建历史区分手动构建与自动构建/发版后构建。
	Origin string `json:"origin,omitempty"`
}

// BatchSubmitRequest queues the same job type for matching projects in a workspace.
type BatchSubmitRequest struct {
	WorkspaceRecID string  `json:"workspace_rec_id"`
	JobType        string  `json:"job_type"`
	Payload        Payload `json:"payload"`
}

// BatchSubmitResult is how many jobs were queued versus skipped.
type BatchSubmitResult struct {
	Queued  int      `json:"queued"`
	Skipped int      `json:"skipped"`
	JobIDs  []string `json:"job_ids,omitempty"`
}

// Runner 是作业执行面。控制面只入队与读日志，不跑步骤。
type Runner interface {
	Enqueue(id string)
	ReadLog(id string, offset int64) (text string, size int64, used int64, err error)
}

// ForcePush reports whether branch_clone should overwrite an existing remote target.
// Missing force defaults to true.
func (p Payload) ForcePush() bool {
	if p.Force == nil {
		return true
	}
	return *p.Force
}

// DecodePayload parses AP_JOB.payload_json. Used by the runner.
func DecodePayload(raw string) Payload {
	var p Payload
	_ = json.Unmarshal([]byte(raw), &p)
	return p
}

// NowUTC returns the UTC timestamp used on AP_JOB rows.
func NowUTC() string {
	return time.Now().UTC().Format(TimeLayout)
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

func encodePayload(p Payload) string {
	b, _ := json.Marshal(p)
	return string(b)
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
