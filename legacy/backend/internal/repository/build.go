package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const (
	BuildStatusPending   = "pending"
	BuildStatusRunning   = "running"
	BuildStatusSuccess   = "success"
	BuildStatusFailed    = "failed"
	BuildStatusCancelled = "cancelled"
)

var ErrBuildNotFound = errors.New("build not found")
var ErrBuildRunning = errors.New("a build is already running for this project")

// PackBuild is a row in AP_PACK_BUILD.
type PackBuild struct {
	RecID        string     `json:"rec_id"`
	ProjectRecID string     `json:"project_rec_id"`
	BuildNum     int        `json:"build_num"`
	Status       string     `json:"status"`
	Message      string     `json:"message"`
	LogPath      string     `json:"log_path"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// BuildRepository provides AP_PACK_BUILD persistence.
type BuildRepository struct {
	db *sql.DB
}

func NewBuild(db *sql.DB) *BuildRepository {
	return &BuildRepository{db: db}
}

// LatestBuildSummary is the most recent build for a project (by build_num).
type LatestBuildSummary struct {
	ProjectRecID string     `json:"project_rec_id"`
	BuildNum     int        `json:"build_num"`
	Status       string     `json:"status"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// LatestByWorkspace returns the latest build per project in the workspace.
func (r *BuildRepository) LatestByWorkspace(workspaceRecID string) (map[string]LatestBuildSummary, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return map[string]LatestBuildSummary{}, nil
	}
	rows, err := r.db.Query(`
		SELECT b.project_rec_id, b.build_num, b.status, b.finished_at, b.created_at
		FROM AP_PACK_BUILD b
		INNER JOIN (
			SELECT project_rec_id, MAX(build_num) AS max_num
			FROM AP_PACK_BUILD
			WHERE project_rec_id IN (
				SELECT rec_id FROM AP_PACK_PROJECT WHERE workspace_rec_id = ?
			)
			GROUP BY project_rec_id
		) latest ON b.project_rec_id = latest.project_rec_id AND b.build_num = latest.max_num`,
		workspaceRecID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]LatestBuildSummary)
	for rows.Next() {
		var item LatestBuildSummary
		var finished sql.NullString
		var created string
		if err := rows.Scan(&item.ProjectRecID, &item.BuildNum, &item.Status, &finished, &created); err != nil {
			return nil, err
		}
		item.FinishedAt = parseNullableTime(finished)
		item.CreatedAt = parseTime(created)
		out[item.ProjectRecID] = item
	}
	return out, rows.Err()
}

func (r *BuildRepository) ListByProject(projectRecID string, limit int) ([]PackBuild, error) {
	projectRecID = strings.TrimSpace(projectRecID)
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := r.db.Query(`
		SELECT rec_id, project_rec_id, build_num, status, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_BUILD
		WHERE project_rec_id = ?
		ORDER BY build_num DESC
		LIMIT ?`, projectRecID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]PackBuild, 0)
	for rows.Next() {
		item, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

// LatestByProject returns the most recent build for a project, or nil if none.
func (r *BuildRepository) LatestByProject(projectRecID string) (*PackBuild, error) {
	projectRecID = strings.TrimSpace(projectRecID)
	if projectRecID == "" {
		return nil, nil
	}
	row := r.db.QueryRow(`
		SELECT rec_id, project_rec_id, build_num, status, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_BUILD
		WHERE project_rec_id = ?
		ORDER BY build_num DESC
		LIMIT 1`, projectRecID)
	item, err := scanBuildRow(row)
	if errors.Is(err, ErrBuildNotFound) {
		return nil, nil
	}
	return item, err
}

func (r *BuildRepository) FindByRecID(recID string) (*PackBuild, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, project_rec_id, build_num, status, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_BUILD WHERE rec_id = ?`, recID)
	return scanBuildRow(row)
}

func (r *BuildRepository) FindRunningByProject(projectRecID string) (*PackBuild, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, project_rec_id, build_num, status, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_BUILD
		WHERE project_rec_id = ? AND status IN (?, ?)
		ORDER BY build_num DESC LIMIT 1`,
		projectRecID, BuildStatusPending, BuildStatusRunning)
	item, err := scanBuildRow(row)
	if errors.Is(err, ErrBuildNotFound) {
		return nil, nil
	}
	return item, err
}

func (r *BuildRepository) NextBuildNum(projectRecID string) (int, error) {
	var n sql.NullInt64
	err := r.db.QueryRow(`
		SELECT MAX(build_num) FROM AP_PACK_BUILD WHERE project_rec_id = ?`, projectRecID).Scan(&n)
	if err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

func (r *BuildRepository) Insert(item *PackBuild) error {
	_, err := r.db.Exec(`
		INSERT INTO AP_PACK_BUILD(
			rec_id, project_rec_id, build_num, status, message, log_path, started_at, finished_at)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		item.RecID, item.ProjectRecID, item.BuildNum, item.Status, item.Message, item.LogPath,
		nullableTime(item.StartedAt), nullableTime(item.FinishedAt),
	)
	return err
}

const defaultInterruptedBuildMessage = "interrupted: build runner not active (server restarted)"

// RecoverInterrupted marks pending/running builds as failed after process exit or restart.
func (r *BuildRepository) RecoverInterrupted(message string) (int64, error) {
	if strings.TrimSpace(message) == "" {
		message = defaultInterruptedBuildMessage
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := r.db.Exec(`
		UPDATE AP_PACK_BUILD SET
			status = ?,
			message = ?,
			finished_at = COALESCE(finished_at, ?)
		WHERE status IN (?, ?)`,
		BuildStatusFailed, message, now,
		BuildStatusPending, BuildStatusRunning,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// ListOlderThan returns builds for a project with created_at strictly before cutoff.
func (r *BuildRepository) ListOlderThan(projectRecID string, before time.Time) ([]PackBuild, error) {
	projectRecID = strings.TrimSpace(projectRecID)
	if projectRecID == "" {
		return nil, nil
	}
	cutoff := before.UTC().Format(time.RFC3339)
	rows, err := r.db.Query(`
		SELECT rec_id, project_rec_id, build_num, status, message, log_path,
			started_at, finished_at, created_at
		FROM AP_PACK_BUILD
		WHERE project_rec_id = ? AND created_at < ?
		ORDER BY build_num ASC`, projectRecID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]PackBuild, 0)
	for rows.Next() {
		item, err := scanBuild(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

func (r *BuildRepository) FindRunningByWorkspace(workspaceRecID string) (*PackBuild, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return nil, nil
	}
	row := r.db.QueryRow(`
		SELECT b.rec_id, b.project_rec_id, b.build_num, b.status, b.message, b.log_path,
			b.started_at, b.finished_at, b.created_at
		FROM AP_PACK_BUILD b
		INNER JOIN AP_PACK_PROJECT p ON p.rec_id = b.project_rec_id
		WHERE p.workspace_rec_id = ? AND b.status IN (?, ?)
		ORDER BY b.build_num DESC
		LIMIT 1`,
		workspaceRecID, BuildStatusPending, BuildStatusRunning)
	item, err := scanBuildRow(row)
	if errors.Is(err, ErrBuildNotFound) {
		return nil, nil
	}
	return item, err
}

func (r *BuildRepository) DeleteByWorkspace(workspaceRecID string) error {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return nil
	}
	_, err := r.db.Exec(`
		DELETE FROM AP_PACK_BUILD
		WHERE project_rec_id IN (
			SELECT rec_id FROM AP_PACK_PROJECT WHERE workspace_rec_id = ?
		)`, workspaceRecID)
	return err
}

func (r *BuildRepository) Delete(recID string) error {
	res, err := r.db.Exec(`DELETE FROM AP_PACK_BUILD WHERE rec_id = ?`, recID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrBuildNotFound
	}
	return nil
}

func (r *BuildRepository) UpdateStatus(recID, status, message string, started, finished *time.Time) error {
	res, err := r.db.Exec(`
		UPDATE AP_PACK_BUILD SET
			status = ?, message = ?,
			started_at = COALESCE(?, started_at),
			finished_at = COALESCE(?, finished_at)
		WHERE rec_id = ?`,
		status, strings.TrimSpace(message), nullableTime(started), nullableTime(finished), recID,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrBuildNotFound
	}
	return nil
}

func nullableTime(t *time.Time) any {
	if t == nil {
		return nil
	}
	return t.UTC().Format(time.RFC3339)
}

type buildScanner interface {
	Scan(dest ...any) error
}

func scanBuild(sc buildScanner) (*PackBuild, error) {
	var item PackBuild
	var started, finished sql.NullString
	var created string
	err := sc.Scan(
		&item.RecID, &item.ProjectRecID, &item.BuildNum, &item.Status, &item.Message, &item.LogPath,
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

func scanBuildRow(row *sql.Row) (*PackBuild, error) {
	item, err := scanBuild(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBuildNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func parseNullableTime(v sql.NullString) *time.Time {
	if !v.Valid || strings.TrimSpace(v.String) == "" {
		return nil
	}
	t := parseTime(v.String)
	return &t
}
