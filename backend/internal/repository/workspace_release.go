package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	ReleaseStatusRunning = "running"
	ReleaseStatusSuccess = "success"
	ReleaseStatusPartial = "partial"
	ReleaseStatusFailed  = "failed"
)

var ErrWorkspaceReleaseNotFound = errors.New("workspace release not found")

// PackWorkspaceRelease is a row in AP_PACK_WORKSPACE_RELEASE.
type PackWorkspaceRelease struct {
	RecID          string     `json:"rec_id"`
	WorkspaceRecID string     `json:"workspace_rec_id"`
	ReleaseNum     int        `json:"release_num"`
	Status         string     `json:"status"`
	ReleaseBranch  string     `json:"release_branch"`
	TagBranch      string     `json:"tag_branch"`
	TargetBranch   string     `json:"target_branch"`
	Released       int        `json:"released"`
	Skipped        int        `json:"skipped"`
	Failed         int        `json:"failed"`
	Message        string     `json:"message"`
	LogPath        string     `json:"log_path"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// WorkspaceReleaseRepository provides AP_PACK_WORKSPACE_RELEASE persistence.
type WorkspaceReleaseRepository struct {
	db *sql.DB
}

func NewWorkspaceRelease(db *sql.DB) *WorkspaceReleaseRepository {
	return &WorkspaceReleaseRepository{db: db}
}

func (r *WorkspaceReleaseRepository) NextReleaseNum(workspaceRecID string) (int, error) {
	var n sql.NullInt64
	err := r.db.QueryRow(`
		SELECT MAX(release_num) FROM AP_PACK_WORKSPACE_RELEASE WHERE workspace_rec_id = ?`,
		workspaceRecID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

func (r *WorkspaceReleaseRepository) Insert(item *PackWorkspaceRelease) error {
	_, err := r.db.Exec(`
		INSERT INTO AP_PACK_WORKSPACE_RELEASE(
			rec_id, workspace_rec_id, release_num, status,
			release_branch, tag_branch, target_branch,
			released, skipped, failed, message, log_path,
			started_at, finished_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.RecID, item.WorkspaceRecID, item.ReleaseNum, item.Status,
		item.ReleaseBranch, item.TagBranch, item.TargetBranch,
		item.Released, item.Skipped, item.Failed, item.Message, item.LogPath,
		nullableTime(item.StartedAt), nullableTime(item.FinishedAt),
	)
	return err
}

func (r *WorkspaceReleaseRepository) FindByRecID(recID string) (*PackWorkspaceRelease, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, workspace_rec_id, release_num, status,
			release_branch, tag_branch, target_branch,
			released, skipped, failed, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_WORKSPACE_RELEASE WHERE rec_id = ?`, recID)
	return scanWorkspaceReleaseRow(row)
}

func (r *WorkspaceReleaseRepository) ListByWorkspace(workspaceRecID string, limit int) ([]PackWorkspaceRelease, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	rows, err := r.db.Query(`
		SELECT rec_id, workspace_rec_id, release_num, status,
			release_branch, tag_branch, target_branch,
			released, skipped, failed, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_WORKSPACE_RELEASE
		WHERE workspace_rec_id = ?
		ORDER BY release_num DESC
		LIMIT ?`, workspaceRecID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PackWorkspaceRelease
	for rows.Next() {
		item, err := scanWorkspaceRelease(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *WorkspaceReleaseRepository) Finish(
	recID, status, message, logPath string,
	released, skipped, failed int,
	finished *time.Time,
) error {
	res, err := r.db.Exec(`
		UPDATE AP_PACK_WORKSPACE_RELEASE SET
			status = ?, message = ?, log_path = COALESCE(NULLIF(?, ''), log_path),
			released = ?, skipped = ?, failed = ?,
			finished_at = COALESCE(?, finished_at)
		WHERE rec_id = ?`,
		status, strings.TrimSpace(message), logPath,
		released, skipped, failed,
		nullableTime(finished), recID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrWorkspaceReleaseNotFound
	}
	return nil
}

const defaultInterruptedReleaseMessage = "interrupted: release runner not active (server restarted)"

// RecoverInterrupted marks running releases as failed after process exit or restart.
func (r *WorkspaceReleaseRepository) RecoverInterrupted(message string) (int64, error) {
	if strings.TrimSpace(message) == "" {
		message = defaultInterruptedReleaseMessage
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`
		UPDATE AP_PACK_WORKSPACE_RELEASE SET
			status = ?,
			message = ?,
			finished_at = COALESCE(finished_at, ?)
		WHERE status = ?`,
		ReleaseStatusFailed, message, now, ReleaseStatusRunning,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ResolveReleaseStatus maps aggregate counts to a stored status.
func ResolveReleaseStatus(released, skipped, failed int) string {
	if failed == 0 {
		return ReleaseStatusSuccess
	}
	if released == 0 {
		return ReleaseStatusFailed
	}
	return ReleaseStatusPartial
}

type workspaceReleaseScanner interface {
	Scan(dest ...any) error
}

func scanWorkspaceRelease(sc workspaceReleaseScanner) (*PackWorkspaceRelease, error) {
	var item PackWorkspaceRelease
	var started, finished sql.NullString
	var created string
	err := sc.Scan(
		&item.RecID, &item.WorkspaceRecID, &item.ReleaseNum, &item.Status,
		&item.ReleaseBranch, &item.TagBranch, &item.TargetBranch,
		&item.Released, &item.Skipped, &item.Failed, &item.Message, &item.LogPath,
		&started, &finished, &created,
	)
	if err != nil {
		return nil, err
	}
	item.StartedAt = parseNullableTime(started)
	item.FinishedAt = parseNullableTime(finished)
	item.CreatedAt = parseTime(created)
	return &item, nil
}

func scanWorkspaceReleaseRow(row *sql.Row) (*PackWorkspaceRelease, error) {
	item, err := scanWorkspaceRelease(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkspaceReleaseNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}
