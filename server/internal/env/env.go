package env

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/crypt"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/mqlog"
	"auto-pack/server/internal/store"
)

// Env is a global environment credential (secrets encrypted).
// workspace_rec_id 保留列兼容旧库，写入时固定为空，不按空间隔离。
type Env struct {
	RecID          string         `json:"rec_id"`
	WorkspaceRecID string         `json:"workspace_rec_id,omitempty"`
	EnvKind        string         `json:"env_kind"`
	EnvName        string         `json:"env_name"`
	ConfigHint     string         `json:"config_hint,omitempty"`
	IsEnabled      int            `json:"is_enabled"`
	Config         map[string]any `json:"config,omitempty"`
	SecretsSet     []string       `json:"secrets_set,omitempty"`
}

// Service manages env vault rows.
type Service struct {
	st       *store.Store
	guard    *iam.Guard
	cipher   *crypt.Cipher
	messages *mqlog.Log
}

// SetMQLog 记录手工生产、消费和查看。未设置时不落库。
func (s *Service) SetMQLog(log *mqlog.Log) {
	if s == nil {
		return
	}
	s.messages = log
}

// ListMQMessages 返回该环境最近 300 条队列收发记录。
func (s *Service) ListMQMessages(ctx context.Context, actor iam.Actor, envRecID string) ([]mqlog.Row, error) {
	if err := s.authorizeRead(ctx, actor, ""); err != nil {
		return nil, err
	}
	if s.messages == nil {
		return []mqlog.Row{}, nil
	}
	return s.messages.List(ctx, envRecID)
}

func (s *Service) noteMQ(ctx context.Context, in mqlog.Entry) {
	if s == nil || s.messages == nil {
		return
	}
	if err := s.messages.Append(ctx, in); err != nil {
		log.Printf("env mqlog: %v", err)
	}
}

// NewService returns an env Service.
func NewService(st *store.Store, guard *iam.Guard, cipher *crypt.Cipher) *Service {
	return &Service{st: st, guard: guard, cipher: cipher}
}

// ListKinds returns the editor catalog (no workspace).
func (s *Service) ListKinds() []KindSpec {
	return Catalog
}

// List returns all env rows without secrets. SDK 环境是全局的，不按工作空间过滤。
func (s *Service) List(ctx context.Context, actor iam.Actor, workspaceID string) ([]Env, error) {
	if err := s.authorizeRead(ctx, actor, workspaceID); err != nil {
		return nil, err
	}
	q := s.st.Rebind(`SELECT rec_id, workspace_rec_id, env_kind, env_name, COALESCE(config_hint,''), is_enabled
		FROM AP_ENV ORDER BY env_kind, env_name`)
	return s.scanList(ctx, q)
}

func (s *Service) authorizeRead(ctx context.Context, actor iam.Actor, workspaceID string) error {
	if ws := strings.TrimSpace(workspaceID); ws != "" {
		if err := s.guard.CanRead(ctx, actor, ws); err == nil {
			return nil
		}
	}
	return s.guard.CanRead(ctx, actor, access.DefaultWorkspaceRecID)
}

func (s *Service) authorizeWrite(ctx context.Context, actor iam.Actor, workspaceID string) error {
	if ws := strings.TrimSpace(workspaceID); ws != "" {
		if err := s.guard.Allow(ctx, actor, ws, iam.ActionEnvWrite); err == nil {
			return nil
		}
	}
	return s.guard.Allow(ctx, actor, access.DefaultWorkspaceRecID, iam.ActionEnvWrite)
}

func (s *Service) scanList(ctx context.Context, q string, args ...any) ([]Env, error) {
	rows, err := s.st.SQL().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Env{}
	for rows.Next() {
		var e Env
		if err := rows.Scan(&e.RecID, &e.WorkspaceRecID, &e.EnvKind, &e.EnvName, &e.ConfigHint, &e.IsEnabled); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// Get returns one env with public config only (secrets stripped).
func (s *Service) Get(ctx context.Context, actor iam.Actor, recID string) (Env, error) {
	e, cipherText, err := s.loadRow(ctx, recID)
	if err != nil {
		return Env{}, err
	}
	if err := s.authorizeRead(ctx, actor, e.WorkspaceRecID); err != nil {
		return Env{}, err
	}
	cfg, err := s.decryptMap(cipherText)
	if err != nil {
		return Env{}, err
	}
	e.Config, e.SecretsSet = Redact(e.EnvKind, cfg)
	return e, nil
}

// Save writes an env (env.write). Secret fields in config are encrypted as a JSON blob.
// Empty secret fields on update keep the stored ciphertext values.
func (s *Service) Save(ctx context.Context, actor iam.Actor, e Env) (Env, error) {
	if err := s.authorizeWrite(ctx, actor, e.WorkspaceRecID); err != nil {
		return Env{}, err
	}
	e.WorkspaceRecID = ""
	kind, err := NormalizeKind(e.EnvKind)
	if err != nil {
		return Env{}, err
	}
	e.EnvKind = kind
	e.EnvName = strings.TrimSpace(e.EnvName)
	if e.EnvName == "" {
		return Env{}, errors.New("env_name required")
	}
	cfg, err := NormalizeConfig(kind, e.Config)
	if err != nil {
		return Env{}, err
	}
	insert := e.RecID == ""
	if e.RecID != "" {
		existing, stored, err := s.loadRow(ctx, e.RecID)
		if errors.Is(err, sql.ErrNoRows) {
			insert = true
		} else if err != nil {
			return Env{}, err
		} else {
			existKind, normErr := NormalizeKind(existing.EnvKind)
			if normErr != nil || existKind != kind {
				return Env{}, errors.New("env_kind cannot change")
			}
			old, err := s.decryptMap(stored)
			if err != nil {
				return Env{}, err
			}
			cfg = MergeSecrets(kind, cfg, old)
		}
	}
	if err := ValidateConfig(kind, cfg); err != nil {
		return Env{}, err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return Env{}, err
	}
	cipherText, err := s.cipher.Encrypt(raw)
	if err != nil {
		return Env{}, err
	}
	hint := ConfigHint(kind, cfg)
	if insert {
		if e.RecID == "" {
			e.RecID = access.NewID()
			if e.IsEnabled == 0 {
				e.IsEnabled = 1
			}
		}
		ins := s.st.Rebind(`INSERT INTO AP_ENV (rec_id, workspace_rec_id, env_kind, env_name, config_cipher, config_hint, is_enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?)`)
		_, err = s.st.SQL().ExecContext(ctx, ins, e.RecID, e.WorkspaceRecID, e.EnvKind, e.EnvName, cipherText, hint, e.IsEnabled)
	} else {
		upd := s.st.Rebind(`UPDATE AP_ENV SET workspace_rec_id='', env_kind=?, env_name=?, config_cipher=?, config_hint=?, is_enabled=? WHERE rec_id=?`)
		_, err = s.st.SQL().ExecContext(ctx, upd, e.EnvKind, e.EnvName, cipherText, hint, e.IsEnabled, e.RecID)
	}
	if err != nil {
		return Env{}, err
	}
	e.Config = nil
	e.SecretsSet = nil
	e.ConfigHint = hint
	return e, nil
}

// Delete removes an env (env.write).
func (s *Service) Delete(ctx context.Context, actor iam.Actor, recID string) error {
	e, _, err := s.loadRow(ctx, recID)
	if err != nil {
		return err
	}
	if err := s.authorizeWrite(ctx, actor, e.WorkspaceRecID); err != nil {
		return err
	}
	q := s.st.Rebind(`DELETE FROM AP_ENV WHERE rec_id = ?`)
	_, err = s.st.SQL().ExecContext(ctx, q, recID)
	return err
}

// DecryptRaw returns plaintext config for the job engine (no HTTP).
func (s *Service) DecryptRaw(ctx context.Context, recID string) (map[string]any, error) {
	_, cfg, err := s.DecryptRow(ctx, recID)
	return cfg, err
}

// DecryptRow returns the stored env row plus plaintext config (engine).
func (s *Service) DecryptRow(ctx context.Context, recID string) (Env, map[string]any, error) {
	e, cipherText, err := s.loadRow(ctx, recID)
	if err != nil {
		return Env{}, nil, err
	}
	m, err := s.decryptMap(cipherText)
	if err != nil {
		return Env{}, nil, err
	}
	return e, m, nil
}

// FindKind returns the first enabled env of a kind (global vault).
func (s *Service) FindKind(ctx context.Context, workspaceID, kind string) (Env, map[string]any, error) {
	_ = workspaceID
	codes := KindCodes(kind)
	if len(codes) == 0 {
		return Env{}, nil, sql.ErrNoRows
	}
	holders := strings.TrimRight(strings.Repeat("?,", len(codes)), ",")
	args := make([]any, 0, len(codes))
	for _, c := range codes {
		args = append(args, c)
	}
	q := s.st.Rebind(`SELECT rec_id, workspace_rec_id, env_kind, env_name, COALESCE(config_hint,''), is_enabled, config_cipher
		FROM AP_ENV WHERE env_kind IN (` + holders + `) AND is_enabled = 1 ORDER BY env_name LIMIT 1`)
	e, cipherText, err := s.scanKindRow(ctx, q, args...)
	if err != nil {
		return Env{}, nil, err
	}
	m, err := s.decryptMap(cipherText)
	if err != nil {
		return Env{}, nil, err
	}
	return e, m, nil
}

// ListKindEnabled 返回该类型全部已启用环境及解密后的配置，按名称、主键排序。
// 消息队列接入要同时消费每一条，不能复用 FindKind 的 LIMIT 1。
func (s *Service) ListKindEnabled(ctx context.Context, kind string) ([]Env, []map[string]any, error) {
	codes := KindCodes(kind)
	if len(codes) == 0 {
		return nil, nil, nil
	}
	holders := strings.TrimRight(strings.Repeat("?,", len(codes)), ",")
	args := make([]any, 0, len(codes))
	for _, c := range codes {
		args = append(args, c)
	}
	q := s.st.Rebind(`SELECT rec_id, workspace_rec_id, env_kind, env_name, COALESCE(config_hint,''), is_enabled, config_cipher
		FROM AP_ENV WHERE env_kind IN (` + holders + `) AND is_enabled = 1 ORDER BY env_name, rec_id`)
	rows, err := s.st.SQL().QueryContext(ctx, q, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var envs []Env
	var cfgs []map[string]any
	for rows.Next() {
		var e Env
		var cipherText string
		if err := rows.Scan(&e.RecID, &e.WorkspaceRecID, &e.EnvKind, &e.EnvName, &e.ConfigHint, &e.IsEnabled, &cipherText); err != nil {
			return nil, nil, err
		}
		cfg, err := s.decryptMap(cipherText)
		if err != nil {
			return nil, nil, err
		}
		envs = append(envs, e)
		cfgs = append(cfgs, cfg)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	return envs, cfgs, nil
}

func (s *Service) scanKindRow(ctx context.Context, q string, args ...any) (Env, string, error) {
	var e Env
	var cipherText string
	err := s.st.SQL().QueryRowContext(ctx, q, args...).Scan(
		&e.RecID, &e.WorkspaceRecID, &e.EnvKind, &e.EnvName, &e.ConfigHint, &e.IsEnabled, &cipherText)
	if errors.Is(err, sql.ErrNoRows) {
		return Env{}, "", err
	}
	return e, cipherText, err
}

func (s *Service) loadRow(ctx context.Context, recID string) (Env, string, error) {
	q := s.st.Rebind(`SELECT rec_id, workspace_rec_id, env_kind, env_name, COALESCE(config_hint,''), is_enabled, config_cipher
		FROM AP_ENV WHERE rec_id = ?`)
	var e Env
	var cipherText string
	err := s.st.SQL().QueryRowContext(ctx, q, recID).Scan(
		&e.RecID, &e.WorkspaceRecID, &e.EnvKind, &e.EnvName, &e.ConfigHint, &e.IsEnabled, &cipherText)
	if errors.Is(err, sql.ErrNoRows) {
		return Env{}, "", err
	}
	return e, cipherText, err
}

func (s *Service) decryptMap(cipherText string) (map[string]any, error) {
	plain, err := s.cipher.Decrypt(cipherText)
	if err != nil {
		return nil, err
	}
	if len(plain) == 0 {
		return map[string]any{}, nil
	}
	var m map[string]any
	if err := json.Unmarshal(plain, &m); err != nil {
		return nil, err
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}
