package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	BranchCloneStatusRunning = ReleaseStatusRunning
	BranchCloneStatusSuccess = ReleaseStatusSuccess
	BranchCloneStatusPartial = ReleaseStatusPartial
	BranchCloneStatusFailed  = ReleaseStatusFailed
)

var ErrWorkspaceBranchCloneNotFound = errors.New("workspace branch clone batch not found")

// PackWorkspaceBranchClone is a row in AP_PACK_WORKSPACE_BRANCH_CLONE.
type PackWorkspaceBranchClone struct {
	RecID          string     `json:"rec_id"`
	WorkspaceRecID string     `json:"workspace_rec_id"`
	BatchNum       int        `json:"batch_num"`
	Status         string     `json:"status"`
	SourceBranch   string     `json:"source_branch"`
	TargetBranch   string     `json:"target_branch"`
	Cloned         int        `json:"cloned"`
	Skipped        int        `json:"skipped"`
	Failed         int        `json:"failed"`
	Message        string     `json:"message"`
	LogPath        string     `json:"log_path"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// WorkspaceBranchCloneRepository provides AP_PACK_WORKSPACE_BRANCH_CLONE persistence.
type WorkspaceBranchCloneRepository struct {
	db *sql.DB
}

func NewWorkspaceBranchClone(db *sql.DB) *WorkspaceBranchCloneRepository {
	return &WorkspaceBranchCloneRepository{db: db}
}

func (r *WorkspaceBranchCloneRepository) NextBatchNum(workspaceRecID string) (int, error) {
	var n sql.NullInt64
	err := r.db.QueryRow(`
		SELECT MAX(batch_num) FROM AP_PACK_WORKSPACE_BRANCH_CLONE WHERE workspace_rec_id = ?`,
		workspaceRecID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

func (r *WorkspaceBranchCloneRepository) Insert(item *PackWorkspaceBranchClone) error {
	_, err := r.db.Exec(`
		INSERT INTO AP_PACK_WORKSPACE_BRANCH_CLONE(
			rec_id, workspace_rec_id, batch_num, status,
			source_branch, target_branch,
			cloned, skipped, failed, message, log_path,
			started_at, finished_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.RecID, item.WorkspaceRecID, item.BatchNum, item.Status,
		item.SourceBranch, item.TargetBranch,
		item.Cloned, item.Skipped, item.Failed, item.Message, item.LogPath,
		nullableTime(item.StartedAt), nullableTime(item.FinishedAt),
	)
	return err
}

func (r *WorkspaceBranchCloneRepository) FindRunningByWorkspace(workspaceRecID string) (*PackWorkspaceBranchClone, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return nil, nil
	}
	row := r.db.QueryRow(`
		SELECT rec_id, workspace_rec_id, batch_num, status,
			source_branch, target_branch,
			cloned, skipped, failed, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_WORKSPACE_BRANCH_CLONE
		WHERE workspace_rec_id = ? AND status = ?
		ORDER BY batch_num DESC
		LIMIT 1`, workspaceRecID, BranchCloneStatusRunning)
	item, err := scanWorkspaceBranchCloneRow(row)
	if errors.Is(err, ErrWorkspaceBranchCloneNotFound) {
		return nil, nil
	}
	return item, err
}

func (r *WorkspaceBranchCloneRepository) FindByRecID(recID string) (*PackWorkspaceBranchClone, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, workspace_rec_id, batch_num, status,
			source_branch, target_branch,
			cloned, skipped, failed, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_WORKSPACE_BRANCH_CLONE WHERE rec_id = ?`, recID)
	return scanWorkspaceBranchCloneRow(row)
}

func (r *WorkspaceBranchCloneRepository) ListByWorkspace(workspaceRecID string, limit int) ([]PackWorkspaceBranchClone, error) {
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
		SELECT rec_id, workspace_rec_id, batch_num, status,
			source_branch, target_branch,
			cloned, skipped, failed, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_WORKSPACE_BRANCH_CLONE
		WHERE workspace_rec_id = ?
		ORDER BY batch_num DESC
		LIMIT ?`, workspaceRecID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PackWorkspaceBranchClone
	for rows.Next() {
		item, err := scanWorkspaceBranchClone(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *WorkspaceBranchCloneRepository) UpdateProgress(recID string, cloned, skipped, failed int) error {
	res, err := r.db.Exec(`
		UPDATE AP_PACK_WORKSPACE_BRANCH_CLONE SET
			cloned = ?, skipped = ?, failed = ?
		WHERE rec_id = ?`,
		cloned, skipped, failed, recID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrWorkspaceBranchCloneNotFound
	}
	return nil
}

func (r *WorkspaceBranchCloneRepository) Finish(
	recID, status, message, logPath string,
	cloned, skipped, failed int,
	finished *time.Time,
) error {
	res, err := r.db.Exec(`
		UPDATE AP_PACK_WORKSPACE_BRANCH_CLONE SET
			status = ?, message = ?, log_path = COALESCE(NULLIF(?, ''), log_path),
			cloned = ?, skipped = ?, failed = ?,
			finished_at = COALESCE(?, finished_at)
		WHERE rec_id = ?`,
		status, strings.TrimSpace(message), logPath,
		cloned, skipped, failed,
		nullableTime(finished), recID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrWorkspaceBranchCloneNotFound
	}
	return nil
}

const defaultInterruptedBranchCloneMessage = "interrupted: branch clone runner not active (server restarted)"

// RecoverInterrupted marks running branch clone batches as failed after process exit or restart.
func (r *WorkspaceBranchCloneRepository) RecoverInterrupted(message string) (int64, error) {
	if strings.TrimSpace(message) == "" {
		message = defaultInterruptedBranchCloneMessage
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`
		UPDATE AP_PACK_WORKSPACE_BRANCH_CLONE SET
			status = ?,
			message = ?,
			finished_at = COALESCE(finished_at, ?)
		WHERE status = ?`,
		BranchCloneStatusFailed, message, now, BranchCloneStatusRunning,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ResolveBranchCloneStatus maps aggregate counts to a stored status.
func ResolveBranchCloneStatus(cloned, skipped, failed int) string {
	return ResolveReleaseStatus(cloned, skipped, failed)
}

type workspaceBranchCloneScanner interface {
	Scan(dest ...any) error
}

func scanWorkspaceBranchClone(sc workspaceBranchCloneScanner) (*PackWorkspaceBranchClone, error) {
	var item PackWorkspaceBranchClone
	var started, finished sql.NullString
	var created string
	err := sc.Scan(
		&item.RecID, &item.WorkspaceRecID, &item.BatchNum, &item.Status,
		&item.SourceBranch, &item.TargetBranch,
		&item.Cloned, &item.Skipped, &item.Failed, &item.Message, &item.LogPath,
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

func scanWorkspaceBranchCloneRow(row *sql.Row) (*PackWorkspaceBranchClone, error) {
	item, err := scanWorkspaceBranchClone(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkspaceBranchCloneNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}
