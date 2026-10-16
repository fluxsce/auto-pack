package settings

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/store"
)

const (
	KeyJobConcurrency   = "job.concurrency"
	KeyLogRetentionDays = "log.retention_days"
	KeyWebTimeoutSec    = "web.timeout_sec"

	DefaultJobConcurrency   = 6
	DefaultLogRetentionDays = 0
	DefaultWebTimeoutSec    = 60

	MaxJobConcurrency   = 16
	MaxLogRetentionDays = 365
	MinWebTimeoutSec    = 5
	MaxWebTimeoutSec    = 600
)

// Props 是平台系统属性。
type Props struct {
	JobConcurrency   int `json:"job_concurrency"`
	LogRetentionDays int `json:"log_retention_days"`
	WebTimeoutSec    int `json:"web_timeout_sec"`
}

// DefaultProps 返回出厂默认（并发 6、不按系统天数清理作业、60s 请求超时）。
func DefaultProps() Props {
	return Props{
		JobConcurrency:   DefaultJobConcurrency,
		LogRetentionDays: DefaultLogRetentionDays,
		WebTimeoutSec:    DefaultWebTimeoutSec,
	}
}

// Snapshot 读库中的系统属性，失败时回默认。供 runner 使用，不鉴权。
func (s *Service) Snapshot(ctx context.Context) Props {
	if s == nil {
		return DefaultProps()
	}
	p, err := s.loadProps(ctx)
	if err != nil {
		return DefaultProps()
	}
	return p
}

// GetProps 返回系统属性（空间成员可读）。
func (s *Service) GetProps(ctx context.Context, actor iam.Actor, workspaceID string) (Props, error) {
	if err := s.requireRead(ctx, actor, workspaceID); err != nil {
		return Props{}, err
	}
	return s.loadProps(ctx)
}

// SaveProps 写入系统属性（空间 owner + settings.write）。
func (s *Service) SaveProps(ctx context.Context, actor iam.Actor, workspaceID string, in Props) (Props, error) {
	if err := s.requireWrite(ctx, actor, workspaceID); err != nil {
		return Props{}, err
	}
	p := ClampProps(in)
	err := s.st.Tx(ctx, func(tx *store.Store) error {
		if err := upsertSetting(ctx, tx, KeyJobConcurrency, strconv.Itoa(p.JobConcurrency)); err != nil {
			return err
		}
		if err := upsertSetting(ctx, tx, KeyLogRetentionDays, strconv.Itoa(p.LogRetentionDays)); err != nil {
			return err
		}
		return upsertSetting(ctx, tx, KeyWebTimeoutSec, strconv.Itoa(p.WebTimeoutSec))
	})
	if err != nil {
		return Props{}, err
	}
	return p, nil
}

// ClampProps 把属性收进合法区间。
func ClampProps(p Props) Props {
	p.JobConcurrency = clampInt(p.JobConcurrency, 1, MaxJobConcurrency)
	p.LogRetentionDays = clampInt(p.LogRetentionDays, 0, MaxLogRetentionDays)
	p.WebTimeoutSec = clampInt(p.WebTimeoutSec, MinWebTimeoutSec, MaxWebTimeoutSec)
	return p
}

func (s *Service) loadProps(ctx context.Context) (Props, error) {
	p := DefaultProps()
	if s == nil || s.st == nil || s.st.SQL() == nil {
		return p, nil
	}
	q := s.st.Rebind(`SELECT setting_key, setting_value FROM AP_SETTING`)
	rows, err := s.st.SQL().QueryContext(ctx, q)
	if err != nil {
		return p, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, val string
		if err := rows.Scan(&key, &val); err != nil {
			return p, err
		}
		n, ok := parseInt(val)
		if !ok {
			continue
		}
		switch key {
		case KeyJobConcurrency:
			p.JobConcurrency = n
		case KeyLogRetentionDays:
			p.LogRetentionDays = n
		case KeyWebTimeoutSec:
			p.WebTimeoutSec = n
		}
	}
	if err := rows.Err(); err != nil {
		return p, err
	}
	return ClampProps(p), nil
}

func upsertSetting(ctx context.Context, st *store.Store, key, value string) error {
	q := st.Rebind(`SELECT rec_id FROM AP_SETTING WHERE setting_key = ?`)
	var id string
	err := st.SQL().QueryRowContext(ctx, q, key).Scan(&id)
	if err == nil && id != "" {
		upd := st.Rebind(`UPDATE AP_SETTING SET setting_value = ?, updated_at = CURRENT_TIMESTAMP WHERE rec_id = ?`)
		_, err = st.SQL().ExecContext(ctx, upd, value, id)
		return err
	}
	ins := st.Rebind(`INSERT INTO AP_SETTING (rec_id, setting_key, setting_value) VALUES (?, ?, ?)`)
	_, err = st.SQL().ExecContext(ctx, ins, access.NewID(), key, value)
	return err
}

func parseInt(s string) (int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}
	return n, true
}

func clampInt(n, lo, hi int) int {
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func (s *Service) requireRead(ctx context.Context, actor iam.Actor, workspaceID string) error {
	if s == nil || s.guard == nil {
		return fmt.Errorf("settings: not configured")
	}
	return s.guard.CanRead(ctx, actor, access.WorkspaceOrDefault(workspaceID))
}

func (s *Service) requireWrite(ctx context.Context, actor iam.Actor, workspaceID string) error {
	if s == nil || s.guard == nil {
		return fmt.Errorf("settings: not configured")
	}
	return s.guard.Allow(ctx, actor, access.WorkspaceOrDefault(workspaceID), iam.ActionSettingsWrite)
}

// AuthorizeWrite 校验 settings.write（系统属性、界面升级等）。
func (s *Service) AuthorizeWrite(ctx context.Context, actor iam.Actor, workspaceID string) error {
	return s.requireWrite(ctx, actor, workspaceID)
}
