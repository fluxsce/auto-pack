package intake

import (
	"context"
	"strings"
	"time"
)

const (
	statusRunning       = "running"
	statusSucceeded     = "succeeded"
	statusFailed        = "failed"
	statusReceiptFailed = "receipt_failed"
)

type taskRow struct {
	TaskID         string
	TaskNo         string
	PackTaskID     string
	SubsystemType  string
	ProjectRecID   string
	JobRecID       string
	TaskStatus     string
	ResultStatus   string
	ResultMessage  string
	OSSProjectRoot string
	NoticeJSON     string
	FinishedAt     string
}

func (s *Service) claim(ctx context.Context, n Notice, raw []byte) (bool, error) {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	q := s.st.Rebind(`INSERT INTO AP_INTAKE_TASK (
		task_id, task_no, pack_task_id, subsystem_type, project_rec_id, job_rec_id,
		task_status, result_status, result_message, oss_project_root, notice_json, finished_at, created_at, updated_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	_, err := s.st.SQL().ExecContext(ctx, q,
		n.TaskID, n.TaskNo, n.PackTaskID, n.SubsystemType, "", "",
		statusRunning, "", "", firstNonEmptyPath(n.OSSProjectRoot, n.FTPProjectRoot), string(raw), "", now, now)
	if err == nil {
		return true, nil
	}
	if isUnique(err) {
		return false, nil
	}
	return false, err
}

func (s *Service) saveTask(ctx context.Context, row taskRow) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	q := s.st.Rebind(`UPDATE AP_INTAKE_TASK SET
		project_rec_id=?, job_rec_id=?, task_status=?, result_status=?, result_message=?,
		oss_project_root=?, finished_at=?, updated_at=? WHERE task_id=?`)
	_, err := s.st.SQL().ExecContext(ctx, q,
		row.ProjectRecID, row.JobRecID, row.TaskStatus, row.ResultStatus, clipMessage(row.ResultMessage),
		row.OSSProjectRoot, row.FinishedAt, now, row.TaskID)
	return err
}

func (s *Service) taskByID(ctx context.Context, taskID string) (taskRow, error) {
	q := s.st.Rebind(`SELECT task_id, COALESCE(task_no,''), COALESCE(pack_task_id,''), COALESCE(subsystem_type,''),
		COALESCE(project_rec_id,''), COALESCE(job_rec_id,''), task_status, COALESCE(result_status,''),
		COALESCE(result_message,''), COALESCE(oss_project_root,''), COALESCE(finished_at,'')
		FROM AP_INTAKE_TASK WHERE task_id=?`)
	var row taskRow
	err := s.st.SQL().QueryRowContext(ctx, q, taskID).Scan(
		&row.TaskID, &row.TaskNo, &row.PackTaskID, &row.SubsystemType,
		&row.ProjectRecID, &row.JobRecID, &row.TaskStatus, &row.ResultStatus,
		&row.ResultMessage, &row.OSSProjectRoot, &row.FinishedAt)
	return row, err
}

func isUnique(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate")
}
