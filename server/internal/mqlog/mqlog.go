// Package mqlog 记录本服务在 MNS 上的生产、消费与查看。每条关联一个消息队列环境，每个环境只保留最近 300 条。
package mqlog

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/store"
)

const (
	Keep = 300

	DirectionProduce = "produce"
	DirectionConsume = "consume"
	DirectionPeek    = "peek"

	SourceIntake = "intake"
	SourceManual = "manual"

	// ActorIntake 是自动接入写入的操作人。
	ActorIntake = "mq"
)

// Entry 是一条待写入的队列消息。
type Entry struct {
	EnvRecID    string
	Direction   string
	QueueRole   string
	QueueName   string
	MessageID   string
	Body        string
	Source      string
	Deleted     bool
	ActorUserID string
}

// Row 是已落库的一条记录，按时间从新到旧返回。
type Row struct {
	RecID       string `json:"rec_id"`
	EnvRecID    string `json:"env_rec_id"`
	SeqNum      int    `json:"seq_num"`
	Direction   string `json:"direction"`
	QueueRole   string `json:"queue_role"`
	QueueName   string `json:"queue_name"`
	MessageID   string `json:"message_id,omitempty"`
	TaskID      string `json:"task_id,omitempty"`
	Body        string `json:"body,omitempty"`
	Source      string `json:"source"`
	Deleted     int    `json:"deleted"`
	ActorUserID string `json:"actor_user_id"`
	CreatedAt   string `json:"created_at"`
}

// Log 读写 AP_MQ_MESSAGE。
type Log struct {
	st *store.Store
}

// New 使用平台库。
func New(st *store.Store) *Log {
	return &Log{st: st}
}

// Append 写入一条并裁到最近 Keep 条。序号冲突时重试。
func (l *Log) Append(ctx context.Context, in Entry) error {
	if l == nil || l.st == nil {
		return nil
	}
	in.EnvRecID = strings.TrimSpace(in.EnvRecID)
	in.Direction = strings.TrimSpace(in.Direction)
	in.QueueRole = strings.TrimSpace(in.QueueRole)
	in.QueueName = strings.TrimSpace(in.QueueName)
	in.Source = strings.TrimSpace(in.Source)
	in.ActorUserID = strings.TrimSpace(in.ActorUserID)
	if in.EnvRecID == "" || in.Direction == "" || in.QueueRole == "" || in.Source == "" || in.ActorUserID == "" {
		return errors.New("mqlog: env, direction, queue, source and actor are required")
	}
	if in.QueueName == "" {
		in.QueueName = in.QueueRole
	}
	var last error
	for i := 0; i < 3; i++ {
		last = l.insert(ctx, in)
		if last == nil || !isSeqConflict(last) {
			return last
		}
	}
	return last
}

func (l *Log) insert(ctx context.Context, in Entry) error {
	return l.st.Tx(ctx, func(tx *store.Store) error {
		var maxSeq int
		if err := tx.SQL().QueryRowContext(ctx, `SELECT COALESCE(MAX(seq_num), 0) FROM AP_MQ_MESSAGE`).Scan(&maxSeq); err != nil {
			return err
		}
		seq := maxSeq + 1
		deleted := 0
		if in.Deleted {
			deleted = 1
		}
		q := tx.Rebind(`INSERT INTO AP_MQ_MESSAGE (
			rec_id, env_rec_id, seq_num, direction, queue_role, queue_name, message_id, task_id, body, source, deleted_flag, actor_user_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if _, err := tx.SQL().ExecContext(ctx, q,
			access.NewID(), in.EnvRecID, seq, in.Direction, in.QueueRole, in.QueueName,
			nullText(in.MessageID), nullText(taskIDOf(in.Body)), nullText(in.Body),
			in.Source, deleted, in.ActorUserID,
		); err != nil {
			return err
		}
		return trimEnv(ctx, tx, in.EnvRecID)
	})
}

func trimEnv(ctx context.Context, tx *store.Store, envID string) error {
	q := tx.Rebind(`SELECT seq_num FROM AP_MQ_MESSAGE WHERE env_rec_id = ? ORDER BY seq_num DESC`)
	rows, err := tx.SQL().QueryContext(ctx, q, envID)
	if err != nil {
		return err
	}
	var drop []int
	n := 0
	for rows.Next() {
		var seq int
		if err := rows.Scan(&seq); err != nil {
			_ = rows.Close()
			return err
		}
		n++
		if n > Keep {
			drop = append(drop, seq)
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if len(drop) == 0 {
		return nil
	}
	del := tx.Rebind(`DELETE FROM AP_MQ_MESSAGE WHERE env_rec_id = ? AND seq_num = ?`)
	for _, seq := range drop {
		if _, err := tx.SQL().ExecContext(ctx, del, envID, seq); err != nil {
			return err
		}
	}
	return nil
}

// List 返回该环境最近的记录，最多 Keep 条。
func (l *Log) List(ctx context.Context, envRecID string) ([]Row, error) {
	if l == nil || l.st == nil {
		return []Row{}, nil
	}
	envRecID = strings.TrimSpace(envRecID)
	if envRecID == "" {
		return []Row{}, nil
	}
	q := l.st.Rebind(`SELECT rec_id, env_rec_id, seq_num, direction, queue_role, queue_name,
		COALESCE(message_id, ''), COALESCE(task_id, ''), COALESCE(body, ''),
		source, deleted_flag, actor_user_id, created_at
		FROM AP_MQ_MESSAGE WHERE env_rec_id = ? ORDER BY seq_num DESC LIMIT ?`)
	rows, err := l.st.SQL().QueryContext(ctx, q, envRecID, Keep)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Row, 0)
	for rows.Next() {
		var row Row
		if err := rows.Scan(&row.RecID, &row.EnvRecID, &row.SeqNum, &row.Direction, &row.QueueRole, &row.QueueName,
			&row.MessageID, &row.TaskID, &row.Body, &row.Source, &row.Deleted, &row.ActorUserID, &row.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func taskIDOf(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return ""
	}
	var probe struct {
		TaskID string `json:"taskId"`
	}
	if err := json.Unmarshal([]byte(body), &probe); err != nil {
		return ""
	}
	return strings.TrimSpace(probe.TaskID)
}

func nullText(s string) any {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}

func isSeqConflict(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "unique") || strings.Contains(s, "duplicate")
}
