package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	PomVersionStatusRunning = ReleaseStatusRunning
	PomVersionStatusSuccess = ReleaseStatusSuccess
	PomVersionStatusPartial = ReleaseStatusPartial
	PomVersionStatusFailed  = ReleaseStatusFailed
)

var ErrWorkspacePomVersionNotFound = errors.New("workspace pom version batch not found")

// PackWorkspacePomVersion is a row in AP_PACK_WORKSPACE_POM_VERSION.
type PackWorkspacePomVersion struct {
	RecID          string     `json:"rec_id"`
	WorkspaceRecID string     `json:"workspace_rec_id"`
	BatchNum       int        `json:"batch_num"`
	Status         string     `json:"status"`
	VersionTag     string     `json:"version_tag"`
	CommitMessage  string     `json:"commit_message"`
	Updated        int        `json:"updated"`
	Skipped        int        `json:"skipped"`
	Failed         int        `json:"failed"`
	Message        string     `json:"message"`
	LogPath        string     `json:"log_path"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	FinishedAt     *time.Time `json:"finished_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// WorkspacePomVersionRepository provides AP_PACK_WORKSPACE_POM_VERSION persistence.
type WorkspacePomVersionRepository struct {
	db *sql.DB
}

func NewWorkspacePomVersion(db *sql.DB) *WorkspacePomVersionRepository {
	return &WorkspacePomVersionRepository{db: db}
}

func (r *WorkspacePomVersionRepository) NextBatchNum(workspaceRecID string) (int, error) {
	var n sql.NullInt64
	err := r.db.QueryRow(`
		SELECT MAX(batch_num) FROM AP_PACK_WORKSPACE_POM_VERSION WHERE workspace_rec_id = ?`,
		workspaceRecID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

func (r *WorkspacePomVersionRepository) Insert(item *PackWorkspacePomVersion) error {
	_, err := r.db.Exec(`
		INSERT INTO AP_PACK_WORKSPACE_POM_VERSION(
			rec_id, workspace_rec_id, batch_num, status,
			version_tag, commit_message,
			updated, skipped, failed, message, log_path,
			started_at, finished_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.RecID, item.WorkspaceRecID, item.BatchNum, item.Status,
		item.VersionTag, item.CommitMessage,
		item.Updated, item.Skipped, item.Failed, item.Message, item.LogPath,
		nullableTime(item.StartedAt), nullableTime(item.FinishedAt),
	)
	return err
}

func (r *WorkspacePomVersionRepository) FindRunningByWorkspace(workspaceRecID string) (*PackWorkspacePomVersion, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return nil, nil
	}
	row := r.db.QueryRow(`
		SELECT rec_id, workspace_rec_id, batch_num, status,
			version_tag, commit_message,
			updated, skipped, failed, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_WORKSPACE_POM_VERSION
		WHERE workspace_rec_id = ? AND status = ?
		ORDER BY batch_num DESC
		LIMIT 1`, workspaceRecID, PomVersionStatusRunning)
	item, err := scanWorkspacePomVersionRow(row)
	if errors.Is(err, ErrWorkspacePomVersionNotFound) {
		return nil, nil
	}
	return item, err
}

func (r *WorkspacePomVersionRepository) FindByRecID(recID string) (*PackWorkspacePomVersion, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, workspace_rec_id, batch_num, status,
			version_tag, commit_message,
			updated, skipped, failed, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_WORKSPACE_POM_VERSION WHERE rec_id = ?`, recID)
	return scanWorkspacePomVersionRow(row)
}

func (r *WorkspacePomVersionRepository) ListByWorkspace(workspaceRecID string, limit int) ([]PackWorkspacePomVersion, error) {
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
			version_tag, commit_message,
			updated, skipped, failed, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_WORKSPACE_POM_VERSION
		WHERE workspace_rec_id = ?
		ORDER BY batch_num DESC
		LIMIT ?`, workspaceRecID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []PackWorkspacePomVersion
	for rows.Next() {
		item, err := scanWorkspacePomVersion(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *item)
	}
	return items, rows.Err()
}

func (r *WorkspacePomVersionRepository) UpdateProgress(recID string, updated, skipped, failed int) error {
	res, err := r.db.Exec(`
		UPDATE AP_PACK_WORKSPACE_POM_VERSION SET
			updated = ?, skipped = ?, failed = ?
		WHERE rec_id = ?`,
		updated, skipped, failed, recID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrWorkspacePomVersionNotFound
	}
	return nil
}

func (r *WorkspacePomVersionRepository) Finish(
	recID, status, message, logPath string,
	updated, skipped, failed int,
	finished *time.Time,
) error {
	res, err := r.db.Exec(`
		UPDATE AP_PACK_WORKSPACE_POM_VERSION SET
			status = ?, message = ?, log_path = COALESCE(NULLIF(?, ''), log_path),
			updated = ?, skipped = ?, failed = ?,
			finished_at = COALESCE(?, finished_at)
		WHERE rec_id = ?`,
		status, strings.TrimSpace(message), logPath,
		updated, skipped, failed,
		nullableTime(finished), recID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrWorkspacePomVersionNotFound
	}
	return nil
}

const defaultInterruptedPomVersionMessage = "interrupted: pom version runner not active (server restarted)"

// RecoverInterrupted marks running pom version batches as failed after process exit or restart.
func (r *WorkspacePomVersionRepository) RecoverInterrupted(message string) (int64, error) {
	if strings.TrimSpace(message) == "" {
		message = defaultInterruptedPomVersionMessage
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`
		UPDATE AP_PACK_WORKSPACE_POM_VERSION SET
			status = ?,
			message = ?,
			finished_at = COALESCE(finished_at, ?)
		WHERE status = ?`,
		PomVersionStatusFailed, message, now, PomVersionStatusRunning,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ResolvePomVersionStatus maps aggregate counts to a stored status.
func ResolvePomVersionStatus(updated, skipped, failed int) string {
	return ResolveReleaseStatus(updated, skipped, failed)
}

type workspacePomVersionScanner interface {
	Scan(dest ...any) error
}

func scanWorkspacePomVersion(sc workspacePomVersionScanner) (*PackWorkspacePomVersion, error) {
	var item PackWorkspacePomVersion
	var started, finished sql.NullString
	var created string
	err := sc.Scan(
		&item.RecID, &item.WorkspaceRecID, &item.BatchNum, &item.Status,
		&item.VersionTag, &item.CommitMessage,
		&item.Updated, &item.Skipped, &item.Failed, &item.Message, &item.LogPath,
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

func scanWorkspacePomVersionRow(row *sql.Row) (*PackWorkspacePomVersion, error) {
	item, err := scanWorkspacePomVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkspacePomVersionNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}
