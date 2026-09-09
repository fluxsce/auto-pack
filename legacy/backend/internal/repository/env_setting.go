package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrEnvNotFound = errors.New("env setting not found")
var ErrEnvNameExists = errors.New("env name already exists")

// EnvSetting is a row in AP_SYS_ENV.
type EnvSetting struct {
	RecID      string    `json:"rec_id"`
	EnvName    string    `json:"env_name"`
	EnvType    string    `json:"env_type"`
	ConfigJSON string    `json:"config_json"`
	IsDefault  int       `json:"is_default"`
	IsEnabled  int       `json:"is_enabled"`
	SortNum    int       `json:"sort_num"`
	Remark     string    `json:"remark"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// EnvRepository provides AP_SYS_ENV persistence.
type EnvRepository struct {
	db *sql.DB
}

func NewEnv(db *sql.DB) *EnvRepository {
	return &EnvRepository{db: db}
}

func (r *EnvRepository) List(envType string) ([]EnvSetting, error) {
	envType = strings.TrimSpace(strings.ToLower(envType))
	q := `
		SELECT rec_id, env_name, env_type, config_json, is_default, is_enabled, sort_num, remark, created_at, updated_at
		FROM AP_SYS_ENV`
	var args []any
	if envType != "" {
		q += ` WHERE env_type = ?`
		args = append(args, envType)
	}
	q += ` ORDER BY env_type ASC, sort_num ASC, env_name ASC`

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []EnvSetting
	for rows.Next() {
		item, err := scanEnv(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

func (r *EnvRepository) FindByRecID(recID string) (*EnvSetting, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, env_name, env_type, config_json, is_default, is_enabled, sort_num, remark, created_at, updated_at
		FROM AP_SYS_ENV WHERE rec_id = ?`, recID)
	return scanEnvRow(row)
}

func (r *EnvRepository) Insert(item *EnvSetting) error {
	_, err := r.db.Exec(`
		INSERT INTO AP_SYS_ENV(rec_id, env_name, env_type, config_json, is_default, is_enabled, sort_num, remark)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		item.RecID, item.EnvName, item.EnvType, item.ConfigJSON, item.IsDefault, item.IsEnabled, item.SortNum, item.Remark,
	)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "unique") {
		return ErrEnvNameExists
	}
	return err
}

func (r *EnvRepository) Update(item *EnvSetting) error {
	res, err := r.db.Exec(`
		UPDATE AP_SYS_ENV SET env_name = ?, env_type = ?, config_json = ?, is_default = ?, is_enabled = ?, sort_num = ?, remark = ?, updated_at = ?
		WHERE rec_id = ?`,
		item.EnvName, item.EnvType, item.ConfigJSON, item.IsDefault, item.IsEnabled, item.SortNum, item.Remark,
		time.Now().UTC().Format("2006-01-02 15:04:05"), item.RecID,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ErrEnvNameExists
		}
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrEnvNotFound
	}
	return nil
}

func (r *EnvRepository) Delete(recID string) error {
	res, err := r.db.Exec(`DELETE FROM AP_SYS_ENV WHERE rec_id = ?`, recID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrEnvNotFound
	}
	return nil
}

// ClearDefaultByType resets is_default for all rows of envType except optional keepRecID.
func (r *EnvRepository) ClearDefaultByType(envType, keepRecID string) error {
	if keepRecID == "" {
		_, err := r.db.Exec(`UPDATE AP_SYS_ENV SET is_default = 0, updated_at = ? WHERE env_type = ? AND is_default = 1`,
			nowTS(), envType)
		return err
	}
	_, err := r.db.Exec(`UPDATE AP_SYS_ENV SET is_default = 0, updated_at = ? WHERE env_type = ? AND is_default = 1 AND rec_id <> ?`,
		nowTS(), envType, keepRecID)
	return err
}

func nowTS() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

type envScanner interface {
	Scan(dest ...any) error
}

func scanEnv(sc envScanner) (*EnvSetting, error) {
	var item EnvSetting
	var created, updated string
	err := sc.Scan(
		&item.RecID, &item.EnvName, &item.EnvType, &item.ConfigJSON,
		&item.IsDefault, &item.IsEnabled, &item.SortNum, &item.Remark,
		&created, &updated,
	)
	if err != nil {
		return nil, err
	}
	item.CreatedAt = parseTime(created)
	item.UpdatedAt = parseTime(updated)
	return &item, nil
}

func scanEnvRow(row *sql.Row) (*EnvSetting, error) {
	item, err := scanEnv(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrEnvNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}

func parseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		return t
	}
	t, _ = time.Parse(time.RFC3339, s)
	return t
}
