package settings

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"runtime"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
)

var (
	// ErrBadKey 表示变量名不合法。
	ErrBadKey = errors.New("settings: invalid var key")
	// ErrReserved 表示试图覆盖 PATH 等保留键。
	ErrReserved = errors.New("settings: reserved var key")
)

var keyPat = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

var reserved = map[string]struct{}{
	"PATH": {}, "PATHEXT": {},
	"SYSTEMROOT": {}, "SYSTEMDRIVE": {}, "WINDIR": {}, "COMSPEC": {},
}

// Var 是一条全局环境变量。
type Var struct {
	RecID     string `json:"rec_id"`
	Key       string `json:"key"`
	Value     string `json:"value"`
	IsEnabled int    `json:"is_enabled"`
	Remark    string `json:"remark,omitempty"`
}

// ListVars 返回全部全局变量（空间成员可读）。
func (s *Service) ListVars(ctx context.Context, actor iam.Actor, workspaceID string) ([]Var, error) {
	if err := s.requireRead(ctx, actor, workspaceID); err != nil {
		return nil, err
	}
	return s.loadVars(ctx, true)
}

// SaveVar 写入或覆盖一条全局变量。
func (s *Service) SaveVar(ctx context.Context, actor iam.Actor, workspaceID string, in Var) (Var, error) {
	if err := s.requireWrite(ctx, actor, workspaceID); err != nil {
		return Var{}, err
	}
	in.Key = strings.TrimSpace(in.Key)
	if err := checkVarKey(in.Key); err != nil {
		return Var{}, err
	}
	if in.IsEnabled != 0 {
		in.IsEnabled = 1
	}
	in.Remark = strings.TrimSpace(in.Remark)
	fold := foldKey(in.Key)
	q := s.st.Rebind(`SELECT rec_id, var_key FROM AP_GLOBAL_VAR`)
	rows, err := s.st.SQL().QueryContext(ctx, q)
	if err != nil {
		return Var{}, err
	}
	var existingID string
	for rows.Next() {
		var id, key string
		if err := rows.Scan(&id, &key); err != nil {
			_ = rows.Close()
			return Var{}, err
		}
		if foldKey(key) == fold {
			existingID = id
			break
		}
	}
	_ = rows.Close()
	if existingID != "" {
		upd := s.st.Rebind(`UPDATE AP_GLOBAL_VAR SET var_key = ?, var_value = ?, is_enabled = ?, remark = ?, updated_at = CURRENT_TIMESTAMP WHERE rec_id = ?`)
		if _, err := s.st.SQL().ExecContext(ctx, upd, in.Key, in.Value, in.IsEnabled, nullIfEmpty(in.Remark), existingID); err != nil {
			return Var{}, err
		}
		in.RecID = existingID
		return in, nil
	}
	in.RecID = access.NewID()
	ins := s.st.Rebind(`INSERT INTO AP_GLOBAL_VAR (rec_id, var_key, var_value, is_enabled, remark) VALUES (?, ?, ?, ?, ?)`)
	if _, err := s.st.SQL().ExecContext(ctx, ins, in.RecID, in.Key, in.Value, in.IsEnabled, nullIfEmpty(in.Remark)); err != nil {
		return Var{}, err
	}
	return in, nil
}

// DeleteVar 删除一条。键不存在不报错。
func (s *Service) DeleteVar(ctx context.Context, actor iam.Actor, workspaceID, key string) error {
	if err := s.requireWrite(ctx, actor, workspaceID); err != nil {
		return err
	}
	key = strings.TrimSpace(key)
	if key == "" {
		return fmt.Errorf("%w: empty", ErrBadKey)
	}
	list, err := s.loadVars(ctx, true)
	if err != nil {
		return err
	}
	fold := foldKey(key)
	for _, it := range list {
		if foldKey(it.Key) != fold {
			continue
		}
		q := s.st.Rebind(`DELETE FROM AP_GLOBAL_VAR WHERE rec_id = ?`)
		_, err = s.st.SQL().ExecContext(ctx, q, it.RecID)
		return err
	}
	return nil
}

// ProcEnv 返回已启用全局变量的 KEY=VALUE，供构建进程叠加。不鉴权。
func (s *Service) ProcEnv(ctx context.Context) []string {
	if s == nil {
		return nil
	}
	list, err := s.loadVars(ctx, false)
	if err != nil {
		return nil
	}
	var out []string
	for _, it := range list {
		if it.IsEnabled == 0 {
			continue
		}
		if err := checkVarKey(it.Key); err != nil {
			continue
		}
		out = append(out, it.Key+"="+it.Value)
	}
	return out
}

func (s *Service) loadVars(ctx context.Context, includeDisabled bool) ([]Var, error) {
	if s == nil || s.st == nil || s.st.SQL() == nil {
		return []Var{}, nil
	}
	q := s.st.Rebind(`SELECT rec_id, var_key, COALESCE(var_value,''), is_enabled, COALESCE(remark,'')
		FROM AP_GLOBAL_VAR ORDER BY var_key`)
	rows, err := s.st.SQL().QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Var{}
	seen := map[string]struct{}{}
	for rows.Next() {
		var it Var
		if err := rows.Scan(&it.RecID, &it.Key, &it.Value, &it.IsEnabled, &it.Remark); err != nil {
			return nil, err
		}
		if err := checkVarKey(it.Key); err != nil {
			continue
		}
		fold := foldKey(it.Key)
		if _, ok := seen[fold]; ok {
			continue
		}
		seen[fold] = struct{}{}
		if !includeDisabled && it.IsEnabled == 0 {
			continue
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func checkVarKey(key string) error {
	if key == "" || !keyPat.MatchString(key) {
		return fmt.Errorf("%w: %s", ErrBadKey, key)
	}
	if _, ok := reserved[strings.ToUpper(key)]; ok {
		return fmt.Errorf("%w: %s", ErrReserved, key)
	}
	return nil
}

func foldKey(key string) string {
	if runtime.GOOS == "windows" {
		return strings.ToUpper(key)
	}
	return key
}

func nullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
