package env

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/crypt"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/store"
)

// Env is a workspace environment credential (secrets encrypted).
type Env struct {
	RecID          string         `json:"rec_id"`
	WorkspaceRecID string         `json:"workspace_rec_id"`
	EnvKind        string         `json:"env_kind"`
	EnvName        string         `json:"env_name"`
	ConfigHint     string         `json:"config_hint,omitempty"`
	IsEnabled      int            `json:"is_enabled"`
	Config         map[string]any `json:"config,omitempty"`
	SecretsSet     []string       `json:"secrets_set,omitempty"`
}

// Service manages env vault rows.
type Service struct {
	st     *store.Store
	guard  *iam.Guard
	cipher *crypt.Cipher
}

// NewService returns an env Service.
func NewService(st *store.Store, guard *iam.Guard, cipher *crypt.Cipher) *Service {
	return &Service{st: st, guard: guard, cipher: cipher}
}

// ListKinds returns the editor catalog (no workspace).
func (s *Service) ListKinds() []KindSpec {
	return Catalog
}

// List returns env rows without secrets (any member).
func (s *Service) List(ctx context.Context, actor iam.Actor, workspaceID string) ([]Env, error) {
	if err := s.guard.CanRead(ctx, actor, workspaceID); err != nil {
		return nil, err
	}
	q := s.st.Rebind(`SELECT rec_id, workspace_rec_id, env_kind, env_name, COALESCE(config_hint,''), is_enabled
		FROM AP_ENV WHERE workspace_rec_id = ? ORDER BY env_kind, env_name`)
	rows, err := s.st.SQL().QueryContext(ctx, q, workspaceID)
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
	if err := s.guard.CanRead(ctx, actor, e.WorkspaceRecID); err != nil {
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
	if err := s.guard.Allow(ctx, actor, e.WorkspaceRecID, iam.ActionEnvWrite); err != nil {
		return Env{}, err
	}
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
	if e.RecID != "" {
		existing, cipherText, err := s.loadRow(ctx, e.RecID)
		if err != nil {
			return Env{}, err
		}
		if existing.WorkspaceRecID != e.WorkspaceRecID {
			return Env{}, errors.New("workspace mismatch")
		}
		existKind, normErr := NormalizeKind(existing.EnvKind)
		if normErr != nil || existKind != kind {
			return Env{}, errors.New("env_kind cannot change")
		}
		old, err := s.decryptMap(cipherText)
		if err != nil {
			return Env{}, err
		}
		cfg = MergeSecrets(kind, cfg, old)
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
	if e.IsEnabled == 0 && e.RecID == "" {
		e.IsEnabled = 1
	}
	if e.RecID == "" {
		e.RecID = access.NewID()
		ins := s.st.Rebind(`INSERT INTO AP_ENV (rec_id, workspace_rec_id, env_kind, env_name, config_cipher, config_hint, is_enabled)
			VALUES (?, ?, ?, ?, ?, ?, ?)`)
		_, err = s.st.SQL().ExecContext(ctx, ins, e.RecID, e.WorkspaceRecID, e.EnvKind, e.EnvName, cipherText, hint, e.IsEnabled)
	} else {
		upd := s.st.Rebind(`UPDATE AP_ENV SET env_kind=?, env_name=?, config_cipher=?, config_hint=?, is_enabled=? WHERE rec_id=?`)
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
	if err := s.guard.Allow(ctx, actor, e.WorkspaceRecID, iam.ActionEnvWrite); err != nil {
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

// FindKind returns the first enabled env of a kind in a workspace (engine).
func (s *Service) FindKind(ctx context.Context, workspaceID, kind string) (Env, map[string]any, error) {
	codes := KindCodes(kind)
	if len(codes) == 0 {
		return Env{}, nil, sql.ErrNoRows
	}
	holders := strings.TrimRight(strings.Repeat("?,", len(codes)), ",")
	args := make([]any, 0, 1+len(codes))
	args = append(args, workspaceID)
	for _, c := range codes {
		args = append(args, c)
	}
	q := s.st.Rebind(`SELECT rec_id, workspace_rec_id, env_kind, env_name, COALESCE(config_hint,''), is_enabled, config_cipher
		FROM AP_ENV WHERE workspace_rec_id = ? AND env_kind IN (` + holders + `) AND is_enabled = 1 ORDER BY env_name LIMIT 1`)
	var e Env
	var cipherText string
	err := s.st.SQL().QueryRowContext(ctx, q, args...).Scan(
		&e.RecID, &e.WorkspaceRecID, &e.EnvKind, &e.EnvName, &e.ConfigHint, &e.IsEnabled, &cipherText)
	if errors.Is(err, sql.ErrNoRows) {
		return Env{}, nil, err
	}
	if err != nil {
		return Env{}, nil, err
	}
	m, err := s.decryptMap(cipherText)
	if err != nil {
		return Env{}, nil, err
	}
	return e, m, nil
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
