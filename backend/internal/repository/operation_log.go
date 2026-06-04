package repository

import (
	"database/sql"
	"strings"
	"time"
)

// OperationLog is a row in AP_SYS_OPERATION_LOG.
type OperationLog struct {
	RecID         string    `json:"rec_id"`
	UserRecID     string    `json:"user_rec_id"`
	LoginName     string    `json:"login_name"`
	Action        string    `json:"action"`
	Module        string    `json:"module"`
	TargetType    string    `json:"target_type"`
	TargetID      string    `json:"target_id"`
	TargetName    string    `json:"target_name"`
	Summary       string    `json:"summary"`
	Detail        string    `json:"detail"`
	RequestMethod string    `json:"request_method"`
	RequestPath   string    `json:"request_path"`
	ClientIP      string    `json:"client_ip"`
	Success       int       `json:"success"`
	ErrorMessage  string    `json:"error_message"`
	CreatedAt     time.Time `json:"created_at"`
}

// OperationLogListFilter filters audit log queries.
type OperationLogListFilter struct {
	Module    string
	Action    string
	UserRecID string
	Keyword   string
	Limit     int
	Offset    int
}

// OperationLogRepository provides AP_SYS_OPERATION_LOG persistence.
type OperationLogRepository struct {
	db *sql.DB
}

func NewOperationLog(db *sql.DB) *OperationLogRepository {
	return &OperationLogRepository{db: db}
}

func (r *OperationLogRepository) Insert(item *OperationLog) error {
	success := 0
	if item.Success == 1 {
		success = 1
	}
	_, err := r.db.Exec(`
		INSERT INTO AP_SYS_OPERATION_LOG(
			rec_id, user_rec_id, login_name, action, module, target_type, target_id, target_name,
			summary, detail, request_method, request_path, client_ip, success, error_message)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.RecID, item.UserRecID, item.LoginName, item.Action, item.Module, item.TargetType,
		item.TargetID, item.TargetName, item.Summary, item.Detail, item.RequestMethod,
		item.RequestPath, item.ClientIP, success, item.ErrorMessage,
	)
	return err
}

func (r *OperationLogRepository) List(f OperationLogListFilter) ([]OperationLog, int, error) {
	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	q := `SELECT rec_id, user_rec_id, login_name, action, module, target_type, target_id, target_name,
		summary, detail, request_method, request_path, client_ip, success, error_message, created_at
		FROM AP_SYS_OPERATION_LOG WHERE 1=1`
	countQ := `SELECT COUNT(*) FROM AP_SYS_OPERATION_LOG WHERE 1=1`
	var args []any
	var countArgs []any

	if m := strings.TrimSpace(f.Module); m != "" {
		q += ` AND module = ?`
		countQ += ` AND module = ?`
		args = append(args, m)
		countArgs = append(countArgs, m)
	}
	if a := strings.TrimSpace(f.Action); a != "" {
		q += ` AND action = ?`
		countQ += ` AND action = ?`
		args = append(args, a)
		countArgs = append(countArgs, a)
	}
	if u := strings.TrimSpace(f.UserRecID); u != "" {
		q += ` AND user_rec_id = ?`
		countQ += ` AND user_rec_id = ?`
		args = append(args, u)
		countArgs = append(countArgs, u)
	}
	if kw := strings.TrimSpace(f.Keyword); kw != "" {
		like := "%" + kw + "%"
		q += ` AND (summary LIKE ? OR target_name LIKE ? OR login_name LIKE ? OR detail LIKE ?)`
		countQ += ` AND (summary LIKE ? OR target_name LIKE ? OR login_name LIKE ? OR detail LIKE ?)`
		for i := 0; i < 4; i++ {
			args = append(args, like)
			countArgs = append(countArgs, like)
		}
	}

	var total int
	if err := r.db.QueryRow(countQ, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	q += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]OperationLog, 0)
	for rows.Next() {
		item, err := scanOperationLog(rows)
		if err != nil {
			return nil, 0, err
		}
		list = append(list, *item)
	}
	return list, total, rows.Err()
}

type operationLogScanner interface {
	Scan(dest ...any) error
}

func scanOperationLog(sc operationLogScanner) (*OperationLog, error) {
	var item OperationLog
	var created string
	err := sc.Scan(
		&item.RecID, &item.UserRecID, &item.LoginName, &item.Action, &item.Module,
		&item.TargetType, &item.TargetID, &item.TargetName, &item.Summary, &item.Detail,
		&item.RequestMethod, &item.RequestPath, &item.ClientIP, &item.Success, &item.ErrorMessage,
		&created,
	)
	if err != nil {
		return nil, err
	}
	item.CreatedAt = parseTime(created)
	return &item, nil
}
