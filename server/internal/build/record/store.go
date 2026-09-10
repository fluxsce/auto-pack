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
	// RecordRunning is AP_WORKSPACE_BUILD.status while a batch is in progress.
	RecordRunning = "running"
	// RecordSuccess is AP_WORKSPACE_BUILD.status when no project failed.
	RecordSuccess = "success"
	// RecordPartial is AP_WORKSPACE_BUILD.status when some projects built and some failed.
	RecordPartial = "partial"
	// RecordFailed is AP_WORKSPACE_BUILD.status when no project built and at least one failed.
	RecordFailed = "failed"
)

// Record is one AP_WORKSPACE_BUILD row.
type Record struct {
	RecID          string `json:"rec_id"`
	WorkspaceRecID string `json:"workspace_rec_id"`
	BuildNum       int    `json:"build_num"`
	Status         string `json:"status"`
	Scope          string `json:"scope,omitempty"`
	Succeeded      int    `json:"succeeded"`
	Skipped        int    `json:"skipped"`
	Failed         int    `json:"failed"`
	Message        string `json:"message,omitempty"`
	LogPath        string `json:"log_path,omitempty"`
	FilterJSON     string `json:"filter_json,omitempty"`
	StartedAt      string `json:"started_at,omitempty"`
	FinishedAt     string `json:"finished_at,omitempty"`
	CreatedAt      string `json:"created_at,omitempty"`
}

const buildCols = `rec_id, workspace_rec_id, build_num, status, COALESCE(scope,''),
		succeeded, skipped, failed, COALESCE(message,''), COALESCE(log_path,''), COALESCE(filter_json,''),
		COALESCE(started_at,''), COALESCE(finished_at,''), COALESCE(created_at,'')`

func scanRecord(s interface{ Scan(dest ...any) error }) (Record, error) {
	var rec Record
	err := s.Scan(&rec.RecID, &rec.WorkspaceRecID, &rec.BuildNum, &rec.Status, &rec.Scope,
		&rec.Succeeded, &rec.Skipped, &rec.Failed, &rec.Message, &rec.LogPath, &rec.FilterJSON,
		&rec.StartedAt, &rec.FinishedAt, &rec.CreatedAt)
	return rec, err
}

// NextNum returns the next build_num for a workspace.
func NextNum(ctx context.Context, st *store.Store, workspaceID string) (int, error) {
	q := st.Rebind(`SELECT MAX(build_num) FROM AP_WORKSPACE_BUILD WHERE workspace_rec_id = ?`)
	var n sql.NullInt64
	if err := st.SQL().QueryRowContext(ctx, q, workspaceID).Scan(&n); err != nil {
		return 0, err
	}
	if !n.Valid {
		return 1, nil
	}
	return int(n.Int64) + 1, nil
}

// Insert writes a new AP_WORKSPACE_BUILD row.
func Insert(ctx context.Context, st *store.Store, rec Record) error {
	q := st.Rebind(`INSERT INTO AP_WORKSPACE_BUILD (
		rec_id, workspace_rec_id, build_num, status, scope,
		succeeded, skipped, failed, message, log_path, filter_json, started_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := st.SQL().ExecContext(ctx, q,
		rec.RecID, rec.WorkspaceRecID, rec.BuildNum, rec.Status, nullStr(rec.Scope),
		rec.Succeeded, rec.Skipped, rec.Failed, nullStr(rec.Message), nullStr(rec.LogPath),
		nullStr(rec.FilterJSON), nullStr(rec.StartedAt),
	)
	return err
}

// Finish marks a build row completed with counts and summary.
func Finish(ctx context.Context, st *store.Store, recID, status, message string, succeeded, skipped, failed int) error {
	q := st.Rebind(`UPDATE AP_WORKSPACE_BUILD SET status=?, message=?, succeeded=?, skipped=?, failed=?, finished_at=?
		WHERE rec_id=?`)
	_, err := st.SQL().ExecContext(ctx, q, status, message, succeeded, skipped, failed, time.Now().UTC().Format("2006-01-02 15:04:05"), recID)
	return err
}

// FindRunning returns the latest running row for a workspace, or a zero Record.
func FindRunning(ctx context.Context, st *store.Store, workspaceID string) (Record, error) {
	q := st.Rebind(`SELECT ` + buildCols + ` FROM AP_WORKSPACE_BUILD
		WHERE workspace_rec_id=? AND status=? ORDER BY build_num DESC LIMIT 1`)
	rec, err := scanRecord(st.SQL().QueryRowContext(ctx, q, workspaceID, RecordRunning))
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, nil
	}
	return rec, err
}

// List returns recent workspace builds, newest first.
func List(ctx context.Context, st *store.Store, workspaceID string, limit int) ([]Record, error) {
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	q := st.Rebind(`SELECT ` + buildCols + ` FROM AP_WORKSPACE_BUILD
		WHERE workspace_rec_id=? ORDER BY build_num DESC LIMIT ?`)
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

// Get returns one AP_WORKSPACE_BUILD row by rec_id.
func Get(ctx context.Context, st *store.Store, recID string) (Record, error) {
	q := st.Rebind(`SELECT ` + buildCols + ` FROM AP_WORKSPACE_BUILD WHERE rec_id=?`)
	rec, err := scanRecord(st.SQL().QueryRowContext(ctx, q, recID))
	if errors.Is(err, sql.ErrNoRows) {
		return Record{}, ErrNotFound
	}
	return rec, err
}

// RecoverInterrupted marks leftover running rows failed after process start.
func RecoverInterrupted(ctx context.Context, st *store.Store) error {
	q := st.Rebind(`UPDATE AP_WORKSPACE_BUILD SET status=?, message=?, finished_at=?
		WHERE status=?`)
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	_, err := st.SQL().ExecContext(ctx, q, RecordFailed, "interrupted: build runner not active (server restarted)", now, RecordRunning)
	return err
}

// ResolveStatus maps batch counts to AP_WORKSPACE_BUILD.status.
func ResolveStatus(succeeded, failed int) string {
	if failed == 0 {
		return RecordSuccess
	}
	if succeeded == 0 {
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
