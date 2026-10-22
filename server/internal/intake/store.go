package intake

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	statusRunning       = "running"
	statusSucceeded     = "succeeded"
	statusFailed        = "failed"
	statusReceiptFailed = "receipt_failed"
)

type taskRow struct {
	RecID          string
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

// claim 记下这一条消息。同一 packTaskId 已经有记录时不再插入，调用方据此跳过打包。行主键是新的 rec_id。
func (s *Service) claim(ctx context.Context, n Notice, raw []byte) (bool, string, error) {
	s.claimMu.Lock()
	defer s.claimMu.Unlock()
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	id := uuid.NewString()
	q := s.st.Rebind(`INSERT INTO AP_INTAKE_TASK (
		rec_id, task_id, task_no, pack_task_id, subsystem_type, project_rec_id, job_rec_id,
		task_status, result_status, result_message, oss_project_root, notice_json, finished_at, created_at, updated_at)
		SELECT ?,?,?,?,?,?,?,?,?,?,?,?,?,?,?
		WHERE NOT EXISTS (SELECT 1 FROM AP_INTAKE_TASK WHERE pack_task_id=?)`)
	res, err := s.st.SQL().ExecContext(ctx, q,
		id, n.TaskID, n.TaskNo, n.PackTaskID, n.SubsystemType, "", "",
		statusRunning, "", "", firstNonEmptyPath(n.OSSProjectRoot, n.FTPProjectRoot), string(raw), nil, now, now,
		n.PackTaskID)
	if err != nil {
		return false, "", err
	}
	nRows, err := res.RowsAffected()
	if err != nil {
		return false, "", err
	}
	if nRows == 0 {
		return false, "", nil
	}
	return true, id, nil
}

func (s *Service) saveTask(ctx context.Context, row taskRow) error {
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	if strings.TrimSpace(row.RecID) == "" {
		return errors.New("intake: rec_id is empty")
	}
	q := s.st.Rebind(`UPDATE AP_INTAKE_TASK SET
		project_rec_id=?, job_rec_id=?, task_status=?, result_status=?, result_message=?,
		oss_project_root=?, finished_at=?, updated_at=? WHERE rec_id=?`)
	_, err := s.st.SQL().ExecContext(ctx, q,
		row.ProjectRecID, row.JobRecID, row.TaskStatus, row.ResultStatus, clipMessage(row.ResultMessage),
		row.OSSProjectRoot, dbTime(row.FinishedAt), now, row.RecID)
	return err
}

func (s *Service) taskByPack(ctx context.Context, packTaskID string) (taskRow, error) {
	q := s.st.Rebind(`SELECT rec_id, task_id, COALESCE(task_no,''), COALESCE(pack_task_id,''), COALESCE(subsystem_type,''),
		COALESCE(project_rec_id,''), COALESCE(job_rec_id,''), task_status, COALESCE(result_status,''),
		COALESCE(result_message,''), COALESCE(oss_project_root,''), COALESCE(finished_at,'')
		FROM AP_INTAKE_TASK WHERE pack_task_id=? ORDER BY updated_at DESC LIMIT 1`)
	var row taskRow
	err := s.st.SQL().QueryRowContext(ctx, q, packTaskID).Scan(
		&row.RecID, &row.TaskID, &row.TaskNo, &row.PackTaskID, &row.SubsystemType,
		&row.ProjectRecID, &row.JobRecID, &row.TaskStatus, &row.ResultStatus,
		&row.ResultMessage, &row.OSSProjectRoot, &row.FinishedAt)
	return row, err
}

func dbTime(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
