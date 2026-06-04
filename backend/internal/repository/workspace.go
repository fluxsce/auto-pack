package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

const DefaultWorkspaceRecID = "00000000-0000-4000-8000-000000000010"

var ErrWorkspaceNotFound = errors.New("workspace not found")
var ErrWorkspaceCodeExists = errors.New("workspace code already exists")
var ErrWorkspaceHasProjects = errors.New("workspace has projects")

// PackWorkspace is a row in AP_PACK_WORKSPACE.
type PackWorkspace struct {
	RecID         string    `json:"rec_id"`
	WorkspaceCode string    `json:"workspace_code"`
	WorkspaceName string    `json:"workspace_name"`
	RootDir       string    `json:"root_dir"`
	IsDefault     int       `json:"is_default"`
	IsEnabled     int       `json:"is_enabled"`
	SortNum       int       `json:"sort_num"`
	Remark        string    `json:"remark"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// WorkspaceRepository provides AP_PACK_WORKSPACE persistence.
type WorkspaceRepository struct {
	db *sql.DB
}

func NewWorkspace(db *sql.DB) *WorkspaceRepository {
	return &WorkspaceRepository{db: db}
}

func (r *WorkspaceRepository) List() ([]PackWorkspace, error) {
	rows, err := r.db.Query(`
		SELECT rec_id, workspace_code, workspace_name, root_dir, is_default, is_enabled, sort_num, remark, created_at, updated_at
		FROM AP_PACK_WORKSPACE
		ORDER BY is_default DESC, sort_num ASC, workspace_name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]PackWorkspace, 0)
	for rows.Next() {
		item, err := scanWorkspace(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

func (r *WorkspaceRepository) FindByRecID(recID string) (*PackWorkspace, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, workspace_code, workspace_name, root_dir, is_default, is_enabled, sort_num, remark, created_at, updated_at
		FROM AP_PACK_WORKSPACE WHERE rec_id = ?`, recID)
	return scanWorkspaceRow(row)
}

func (r *WorkspaceRepository) FindDefault() (*PackWorkspace, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, workspace_code, workspace_name, root_dir, is_default, is_enabled, sort_num, remark, created_at, updated_at
		FROM AP_PACK_WORKSPACE WHERE is_default = 1 LIMIT 1`)
	item, err := scanWorkspaceRow(row)
	if err == nil {
		return item, nil
	}
	if !errors.Is(err, ErrWorkspaceNotFound) {
		return nil, err
	}
	return r.FindByRecID(DefaultWorkspaceRecID)
}

func (r *WorkspaceRepository) CountProjects(recID string) (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM AP_PACK_PROJECT WHERE workspace_rec_id = ?`, recID).Scan(&n)
	return n, err
}

func (r *WorkspaceRepository) Insert(item *PackWorkspace) error {
	_, err := r.db.Exec(`
		INSERT INTO AP_PACK_WORKSPACE(rec_id, workspace_code, workspace_name, root_dir, is_default, is_enabled, sort_num, remark)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
		item.RecID, item.WorkspaceCode, item.WorkspaceName, item.RootDir,
		item.IsDefault, item.IsEnabled, item.SortNum, item.Remark,
	)
	return mapWorkspaceUniqueErr(err)
}

func (r *WorkspaceRepository) Update(item *PackWorkspace) error {
	res, err := r.db.Exec(`
		UPDATE AP_PACK_WORKSPACE SET workspace_code = ?, workspace_name = ?, root_dir = ?,
			is_default = ?, is_enabled = ?, sort_num = ?, remark = ?, updated_at = ?
		WHERE rec_id = ?`,
		item.WorkspaceCode, item.WorkspaceName, item.RootDir,
		item.IsDefault, item.IsEnabled, item.SortNum, item.Remark, nowTS(), item.RecID,
	)
	if err != nil {
		return mapWorkspaceUniqueErr(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrWorkspaceNotFound
	}
	return nil
}

func (r *WorkspaceRepository) Delete(recID string) error {
	n, err := r.CountProjects(recID)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrWorkspaceHasProjects
	}
	res, err := r.db.Exec(`DELETE FROM AP_PACK_WORKSPACE WHERE rec_id = ?`, recID)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		return ErrWorkspaceNotFound
	}
	return nil
}

func (r *WorkspaceRepository) ClearDefault(keepRecID string) error {
	if keepRecID == "" {
		_, err := r.db.Exec(`UPDATE AP_PACK_WORKSPACE SET is_default = 0, updated_at = ? WHERE is_default = 1`, nowTS())
		return err
	}
	_, err := r.db.Exec(`UPDATE AP_PACK_WORKSPACE SET is_default = 0, updated_at = ? WHERE is_default = 1 AND rec_id <> ?`,
		nowTS(), keepRecID)
	return err
}

func mapWorkspaceUniqueErr(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "workspace_code") || strings.Contains(lower, "ix_ap_pack_ws_code") || strings.Contains(lower, "unique") {
		return ErrWorkspaceCodeExists
	}
	return err
}

type workspaceScanner interface {
	Scan(dest ...any) error
}

func scanWorkspace(sc workspaceScanner) (*PackWorkspace, error) {
	var item PackWorkspace
	var created, updated string
	err := sc.Scan(
		&item.RecID, &item.WorkspaceCode, &item.WorkspaceName, &item.RootDir,
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

func scanWorkspaceRow(row *sql.Row) (*PackWorkspace, error) {
	item, err := scanWorkspace(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}
