package job

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/store"
)

func insertJob(ctx context.Context, st *store.Store, j Job) error {
	q := st.Rebind(`INSERT INTO AP_JOB (rec_id, workspace_rec_id, project_rec_id, job_type, job_status, payload_json, created_by)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	_, err := st.SQL().ExecContext(ctx, q, j.RecID, j.WorkspaceRecID, nullStr(j.ProjectRecID), j.JobType, j.JobStatus, j.PayloadJSON, j.CreatedBy)
	return err
}

func insertStep(ctx context.Context, st *store.Store, s Step) error {
	q := st.Rebind(`INSERT INTO AP_JOB_STEP (rec_id, job_rec_id, step_key, step_name, sort_num, step_status)
		VALUES (?, ?, ?, ?, ?, ?)`)
	_, err := st.SQL().ExecContext(ctx, q, s.RecID, s.JobRecID, s.StepKey, s.StepName, s.SortNum, s.StepStatus)
	return err
}

const jobSelectCols = `rec_id, workspace_rec_id, COALESCE(project_rec_id,''), job_type, job_status, payload_json,
		COALESCE(error_text,''), created_by, COALESCE(started_at,''), COALESCE(finished_at,''), created_at`

func scanJob(s interface{ Scan(dest ...any) error }, j *Job) error {
	return s.Scan(&j.RecID, &j.WorkspaceRecID, &j.ProjectRecID, &j.JobType, &j.JobStatus,
		&j.PayloadJSON, &j.ErrorText, &j.CreatedBy, &j.StartedAt, &j.FinishedAt, &j.CreatedAt)
}

func listWhere(q ListQuery) (string, []any) {
	var where []string
	var args []any
	if id := strings.TrimSpace(q.WorkspaceID); id != "" {
		where = append(where, "workspace_rec_id = ?")
		args = append(args, id)
	}
	if id := strings.TrimSpace(q.ProjectID); id != "" {
		where = append(where, "project_rec_id = ?")
		args = append(args, id)
	}
	if typ := strings.TrimSpace(q.JobType); typ != "" {
		where = append(where, "job_type = ?")
		args = append(args, typ)
	}
	if len(where) == 0 {
		return "1=1", nil
	}
	return strings.Join(where, " AND "), args
}

func listJobsPage(ctx context.Context, st *store.Store, q ListQuery) (Page, error) {
	page, size := normalizePage(q.Page, q.PageSize)
	cond, args := listWhere(q)

	countQ := st.Rebind(`SELECT COUNT(1) FROM AP_JOB WHERE ` + cond)
	var total int
	if err := st.SQL().QueryRowContext(ctx, countQ, args...).Scan(&total); err != nil {
		return Page{}, err
	}

	listQ := st.Rebind(`SELECT ` + jobSelectCols + ` FROM AP_JOB WHERE ` + cond + `
		ORDER BY created_at DESC LIMIT ? OFFSET ?`)
	listArgs := append(append([]any{}, args...), size, (page-1)*size)
	items, err := queryJobs(ctx, st, listQ, listArgs...)
	if err != nil {
		return Page{}, err
	}

	liveQ := st.Rebind(`SELECT ` + jobSelectCols + ` FROM AP_JOB WHERE ` + cond + `
		AND job_status IN ('queued','running') ORDER BY created_at DESC`)
	live, err := queryJobs(ctx, st, liveQ, args...)
	if err != nil {
		return Page{}, err
	}

	stats, err := jobStats(ctx, st, cond, args)
	if err != nil {
		return Page{}, err
	}
	if items == nil {
		items = []Job{}
	}
	if live == nil {
		live = []Job{}
	}
	return Page{Items: items, Live: live, Stats: stats, Total: total, Page: page, PageSize: size}, nil
}

func queryJobs(ctx context.Context, st *store.Store, q string, args ...any) ([]Job, error) {
	rows, err := st.SQL().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		var j Job
		if err := scanJob(rows, &j); err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func jobStats(ctx context.Context, st *store.Store, cond string, args []any) (Stats, error) {
	q := st.Rebind(`SELECT job_status, COUNT(1) FROM AP_JOB WHERE ` + cond + ` GROUP BY job_status`)
	rows, err := st.SQL().QueryContext(ctx, q, args...)
	if err != nil {
		return Stats{}, err
	}
	defer rows.Close()
	var s Stats
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return Stats{}, err
		}
		switch status {
		case StatusRunning:
			s.Running = n
		case StatusQueued:
			s.Queued = n
		case StatusSucceeded:
			s.Succeeded = n
		case StatusFailed:
			s.Failed = n
		case StatusCanceled:
			s.Canceled = n
		}
	}
	return s, rows.Err()
}

func newStep(jobID, key, name string, sort int) Step {
	return Step{RecID: access.NewID(), JobRecID: jobID, StepKey: key, StepName: name, SortNum: sort, StepStatus: StepPending}
}

// JobByID 读一条作业。执行面与控制面共用。
func JobByID(ctx context.Context, st *store.Store, id string) (Job, error) {
	q := st.Rebind(`SELECT ` + jobSelectCols + ` FROM AP_JOB WHERE rec_id = ?`)
	var j Job
	err := scanJob(st.SQL().QueryRowContext(ctx, q, id), &j)
	if errors.Is(err, sql.ErrNoRows) {
		return Job{}, err
	}
	return j, err
}

// StepsByJob 按 sort_num 列出步骤。
func StepsByJob(ctx context.Context, st *store.Store, jobID string) ([]Step, error) {
	q := st.Rebind(`SELECT rec_id, job_rec_id, step_key, step_name, sort_num, step_status, COALESCE(error_text,''),
		COALESCE(started_at,''), COALESCE(finished_at,'')
		FROM AP_JOB_STEP WHERE job_rec_id = ? ORDER BY sort_num`)
	rows, err := st.SQL().QueryContext(ctx, q, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Step
	for rows.Next() {
		var s Step
		if err := rows.Scan(&s.RecID, &s.JobRecID, &s.StepKey, &s.StepName, &s.SortNum, &s.StepStatus, &s.ErrorText, &s.StartedAt, &s.FinishedAt); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetJobStatus 更新作业状态与时间戳。
func SetJobStatus(ctx context.Context, st *store.Store, id, status, errText string, started, finished bool) error {
	q := st.Rebind(`UPDATE AP_JOB SET job_status=?, error_text=?, started_at=CASE WHEN ? THEN COALESCE(started_at, ?) ELSE started_at END,
		finished_at=CASE WHEN ? THEN ? ELSE finished_at END WHERE rec_id=?`)
	ts := NowUTC()
	startFlag, finFlag := 0, 0
	if started {
		startFlag = 1
	}
	if finished {
		finFlag = 1
	}
	_, err := st.SQL().ExecContext(ctx, q, status, nullStr(errText), startFlag, ts, finFlag, ts, id)
	return err
}

// SetStepStatus 更新步骤状态与时间戳。
func SetStepStatus(ctx context.Context, st *store.Store, id, status, errText string, started, finished bool) error {
	q := st.Rebind(`UPDATE AP_JOB_STEP SET step_status=?, error_text=?, started_at=CASE WHEN ? THEN COALESCE(started_at, ?) ELSE started_at END,
		finished_at=CASE WHEN ? THEN ? ELSE finished_at END WHERE rec_id=?`)
	ts := NowUTC()
	startFlag, finFlag := 0, 0
	if started {
		startFlag = 1
	}
	if finished {
		finFlag = 1
	}
	_, err := st.SQL().ExecContext(ctx, q, status, nullStr(errText), startFlag, ts, finFlag, ts, id)
	return err
}

// RecoverInterrupted 把进程中断时仍 running 的作业标为失败。
func RecoverInterrupted(ctx context.Context, st *store.Store) error {
	q := st.Rebind(`UPDATE AP_JOB SET job_status=?, error_text=?, finished_at=? WHERE job_status=?`)
	_, err := st.SQL().ExecContext(ctx, q, StatusFailed, "interrupted by process restart", NowUTC(), StatusRunning)
	if err != nil {
		return err
	}
	q2 := st.Rebind(`UPDATE AP_JOB_STEP SET step_status=?, error_text=?, finished_at=? WHERE step_status=?`)
	_, err = st.SQL().ExecContext(ctx, q2, StepFailed, "interrupted by process restart", NowUTC(), StepRunning)
	return err
}

// QueuedIDs 返回待跑作业 id，按创建时间。
func QueuedIDs(ctx context.Context, st *store.Store) ([]string, error) {
	q := `SELECT rec_id FROM AP_JOB WHERE job_status = 'queued' ORDER BY created_at`
	rows, err := st.SQL().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

const (
	defaultPruneBatch = 200
	maxPruneBatch     = 1000
	deleteChunkSize   = 80
)

// RetentionTarget is one project the background prune loop should inspect.
type RetentionTarget struct {
	ProjectID   string
	WorkspaceID string
	RecipeJSON  string
}

// ListRetentionTargets returns every project id, workspace, and recipe for prune.
func ListRetentionTargets(ctx context.Context, st *store.Store) ([]RetentionTarget, error) {
	q := `SELECT rec_id, workspace_rec_id, recipe_json FROM AP_PROJECT`
	rows, err := st.SQL().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RetentionTarget
	for rows.Next() {
		var t RetentionTarget
		if err := rows.Scan(&t.ProjectID, &t.WorkspaceID, &t.RecipeJSON); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ListOrphanWorkspaces lists workspaces that have finished jobs with no project.
func ListOrphanWorkspaces(ctx context.Context, st *store.Store) ([]string, error) {
	q := `SELECT DISTINCT workspace_rec_id FROM AP_JOB
		WHERE (project_rec_id IS NULL OR project_rec_id = '')
		AND finished_at IS NOT NULL AND job_status NOT IN ('queued','running')`
	rows, err := st.SQL().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		if id != "" {
			out = append(out, id)
		}
	}
	return out, rows.Err()
}

// ExpiredFinished lists finished jobs older than cutoff for retention.
// cutoff uses TimeLayout. A non-empty projectID scopes to that project; otherwise
// it lists workspace jobs that have no project. limit caps one batch.
func ExpiredFinished(ctx context.Context, st *store.Store, projectID, workspaceID, cutoff string, limit int) ([]Job, error) {
	cutoff = strings.TrimSpace(cutoff)
	if cutoff == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultPruneBatch
	}
	if limit > maxPruneBatch {
		limit = maxPruneBatch
	}
	var q string
	var args []any
	if id := strings.TrimSpace(projectID); id != "" {
		q = st.Rebind(`SELECT rec_id FROM AP_JOB WHERE project_rec_id = ? AND finished_at IS NOT NULL
			AND finished_at < ? AND job_status NOT IN ('queued','running')
			ORDER BY finished_at LIMIT ?`)
		args = []any{id, cutoff, limit}
	} else if ws := strings.TrimSpace(workspaceID); ws != "" {
		q = st.Rebind(`SELECT rec_id FROM AP_JOB WHERE workspace_rec_id = ?
			AND (project_rec_id IS NULL OR project_rec_id = '') AND finished_at IS NOT NULL
			AND finished_at < ? AND job_status NOT IN ('queued','running')
			ORDER BY finished_at LIMIT ?`)
		args = []any{ws, cutoff, limit}
	} else {
		return nil, nil
	}
	rows, err := st.SQL().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Job
	for rows.Next() {
		var j Job
		if err := rows.Scan(&j.RecID); err != nil {
			return nil, err
		}
		j.ProjectRecID = strings.TrimSpace(projectID)
		j.WorkspaceRecID = strings.TrimSpace(workspaceID)
		out = append(out, j)
	}
	return out, rows.Err()
}

// DeleteFinished removes a completed job and its steps. Queued or running rows are left untouched.
func DeleteFinished(ctx context.Context, st *store.Store, id string) error {
	_, err := DeleteFinishedIDs(ctx, st, []string{id})
	return err
}

// DeleteFinishedIDs deletes finished jobs in chunks. Live jobs in the set are skipped.
func DeleteFinishedIDs(ctx context.Context, st *store.Store, ids []string) (int, error) {
	clean := make([]string, 0, len(ids))
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		clean = append(clean, id)
	}
	if len(clean) == 0 {
		return 0, nil
	}
	removed := 0
	for start := 0; start < len(clean); start += deleteChunkSize {
		end := start + deleteChunkSize
		if end > len(clean) {
			end = len(clean)
		}
		n, err := deleteFinishedChunk(ctx, st, clean[start:end])
		if err != nil {
			return removed, err
		}
		removed += n
	}
	return removed, nil
}

func deleteFinishedChunk(ctx context.Context, st *store.Store, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	var removed int
	err := st.Tx(ctx, func(tx *store.Store) error {
		ph, args := placeholders(ids)
		steps := tx.Rebind(`DELETE FROM AP_JOB_STEP WHERE job_rec_id IN (` + ph + `)`)
		if _, err := tx.SQL().ExecContext(ctx, steps, args...); err != nil {
			return err
		}
		jobs := tx.Rebind(`DELETE FROM AP_JOB WHERE rec_id IN (` + ph + `) AND job_status NOT IN ('queued','running')`)
		res, err := tx.SQL().ExecContext(ctx, jobs, args...)
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		removed = int(n)
		return nil
	})
	return removed, err
}

func placeholders(ids []string) (string, []any) {
	args := make([]any, len(ids))
	var b strings.Builder
	for i, id := range ids {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('?')
		args[i] = id
	}
	return b.String(), args
}
