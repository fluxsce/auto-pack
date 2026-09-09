package record

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"auto-pack/server/internal/store"
)

const (
	// RecordRunning is AP_WORKSPACE_RELEASE.status while a batch is in progress.
	RecordRunning = "running"
	// RecordSuccess is AP_WORKSPACE_RELEASE.status when no project failed.
	RecordSuccess = "success"
	// RecordPartial is AP_WORKSPACE_RELEASE.status when some projects released and some failed.
	RecordPartial = "partial"
	// RecordFailed is AP_WORKSPACE_RELEASE.status when no project released and at least one failed.
	RecordFailed = "failed"
)

// Record is one AP_WORKSPACE_RELEASE row.
type Record struct {
	RecID           string `json:"rec_id"`
	WorkspaceRecID  string `json:"workspace_rec_id"`
	ReleaseNum      int    `json:"release_num"`
	Status          string `json:"status"`
	ReleaseBranch   string `json:"release_branch"`
	TagBranch       string `json:"tag_branch"`
	TargetBranch    string `json:"target_branch,omitempty"`
	Released        int    `json:"released"`
	Skipped         int    `json:"skipped"`
	Failed          int    `json:"failed"`
	Message         string `json:"message,omitempty"`
	LogPath         string `json:"log_path,omitempty"`
	PostOptionsJSON string `json:"post_options_json,omitempty"`
	StartedAt       string `json:"started_at,omitempty"`
	FinishedAt      string `json:"finished_at,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
}

const releaseCols = `rec_id, workspace_rec_id, release_num, status,
		COALESCE(release_branch,''), COALESCE(tag_branch,''), COALESCE(target_branch,''),
		released, skipped, failed, COALESCE(message,''), COALESCE(log_path,''), COALESCE(post_options_json,''),
		COALESCE(started_at,''), COALESCE(finished_at,''), COALESCE(created_at,'')`

func scanRecord(s interface{ Scan(dest ...any) error }) (Record, error) {
	var rec Record
	err := s.Scan(&rec.RecID, &rec.WorkspaceRecID, &rec.ReleaseNum, &rec.Status,
		&rec.ReleaseBranch, &rec.TagBranch, &rec.TargetBranch,
		&rec.Released, &rec.Skipped, &rec.Failed, &rec.Message, &rec.LogPath, &rec.PostOptionsJSON,
		&rec.StartedAt, &rec.FinishedAt, &rec.CreatedAt)
	return rec, err
}

// NextNum returns the next release_num for a workspace.
func NextNum(ctx context.Context, st *store.Store, workspaceID string) (int, error) {
	q := st.Rebind(`SELECT MAX(release_num) FROM AP_WORKSPACE_RELEASE WHERE workspace_rec_id = ?`)
	var n sql.NullInt64
	if err := st.SQL().QueryRowContext(ctx, q, workspaceID).Scan(&n); err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

// Insert writes a new AP_WORKSPACE_RELEASE row.
func Insert(ctx context.Context, st *store.Store, rec Record) error {
	q := st.Rebind(`INSERT INTO AP_WORKSPACE_RELEASE (
		rec_id, workspace_rec_id, release_num, status,
		release_branch, tag_branch, target_branch,
		released, skipped, failed, message, log_path, post_options_json, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := st.SQL().ExecContext(ctx, q,
		rec.RecID, rec.WorkspaceRecID, rec.ReleaseNum, rec.Status,
		nullStr(rec.ReleaseBranch), nullStr(rec.TagBranch), nullStr(rec.TargetBranch),
		rec.Released, rec.Skipped, rec.Failed, nullStr(rec.Message), nullStr(rec.LogPath),
		nullStr(rec.PostOptionsJSON), nullStr(rec.StartedAt),
	)
	return err
}

// Finish marks a release row completed with counts and summary.
func Finish(ctx context.Context, st *store.Store, recID, status, message string, released, skipped, failed int) error {
	q := st.Rebind(`UPDATE AP_WORKSPACE_RELEASE SET status=?, message=?, released=?, skipped=?, failed=?, finished_at=?
		WHERE rec_id=?`)
	_, err := st.SQL().ExecContext(ctx, q, status, message, released, skipped, failed, time.Now().UTC().Format("2006-01-02 15:04:05"), recID)
	return err
}

// UpdatePostOptions writes post_options_json on an existing row.
func UpdatePostOptions(ctx context.Context, st *store.Store, recID, raw string) error {
	q := st.Rebind(`UPDATE AP_WORKSPACE_RELEASE SET post_options_json=? WHERE rec_id=?`)
	res, err := st.SQL().ExecContext(ctx, q, raw, recID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		if _, err := Get(ctx, st, recID); err != nil {
			return err
		}
	}
	return nil
}

// FindRunning returns the latest running row for a workspace, or a zero Record.
func FindRunning(ctx context.Context, st *store.Store, workspaceID string) (Record, error) {
	q := st.Rebind(`SELECT ` + releaseCols + ` FROM AP_WORKSPACE_RELEASE
		WHERE workspace_rec_id=? AND status=? ORDER BY release_num DESC LIMIT 1`)
	rec, err := scanRecord(st.SQL().QueryRowContext(ctx, q, workspaceID, RecordRunning))
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, nil
	}
	return rec, err
}

// List returns recent workspace releases, newest first.
func List(ctx context.Context, st *store.Store, workspaceID string, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	q := st.Rebind(`SELECT ` + releaseCols + ` FROM AP_WORKSPACE_RELEASE
		WHERE workspace_rec_id=? ORDER BY release_num DESC LIMIT ?`)
	rows, err := st.SQL().QueryContext(ctx, q, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Record
	for rows.Next() {
		rec, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

// Get returns one AP_WORKSPACE_RELEASE row by rec_id.
func Get(ctx context.Context, st *store.Store, recID string) (Record, error) {
	q := st.Rebind(`SELECT ` + releaseCols + ` FROM AP_WORKSPACE_RELEASE WHERE rec_id=?`)
	rec, err := scanRecord(st.SQL().QueryRowContext(ctx, q, recID))
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	return rec, err
}

// RecoverInterrupted marks leftover running rows failed after process start.
func RecoverInterrupted(ctx context.Context, st *store.Store) error {
	q := st.Rebind(`UPDATE AP_WORKSPACE_RELEASE SET status=?, message=?, finished_at=?
		WHERE status=?`)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := st.SQL().ExecContext(ctx, q, RecordFailed, "interrupted: release runner not active (server restarted)", now, RecordRunning)
	return err
}

// ListTagHistory returns recent tag_branch values for a workspace.
func ListTagHistory(ctx context.Context, st *store.Store, workspaceID string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 50
	}
	q := st.Rebind(`SELECT COALESCE(tag_branch,'') FROM AP_WORKSPACE_RELEASE
		WHERE workspace_rec_id=? AND COALESCE(tag_branch,'') <> '' ORDER BY release_num DESC LIMIT ?`)
	rows, err := st.SQL().QueryContext(ctx, q, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			return nil, err
		}
		out = append(out, strings.TrimSpace(tag))
	}
	return out, rows.Err()
}

// ResolveStatus maps batch counts to AP_WORKSPACE_RELEASE.status.
func ResolveStatus(released, failed int) string {
	if failed == 0 {
		return RecordSuccess
	}
	if released == 0 {
		return RecordFailed
	}
	return RecordPartial
}

func nullStr(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
