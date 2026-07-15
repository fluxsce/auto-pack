package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"
)

var ErrProjectNotFound = errors.New("project not found")
var ErrProjectVersionExists = errors.New("project name and version already exists in workspace")

// PackProject is a row in AP_PACK_PROJECT.
type PackProject struct {
	RecID          string    `json:"rec_id"`
	WorkspaceRecID string    `json:"workspace_rec_id"`
	ProjectName    string    `json:"project_name"`
	ProjectVersion string    `json:"project_version"`
	ProjectType    string    `json:"project_type"`
	RepoURL        string    `json:"repo_url"`
	RepoBranch     string    `json:"repo_branch"`
	WorkDir        string    `json:"work_dir"`
	ConfigJSON     string    `json:"config_json"`
	GitEnvRecID    string    `json:"git_env_rec_id"`
	JdkEnvRecID    string    `json:"jdk_env_rec_id"`
	MavenEnvRecID  string    `json:"maven_env_rec_id"`
	NodejsEnvRecID string    `json:"nodejs_env_rec_id"`
	DeployEnvRecID string    `json:"deploy_env_rec_id"`
	IsEnabled      int       `json:"is_enabled"`
	SortNum        int       `json:"sort_num"`
	Remark         string    `json:"remark"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// ProjectRepository provides AP_PACK_PROJECT persistence.
type ProjectRepository struct {
	db *sql.DB
}

func NewProject(db *sql.DB) *ProjectRepository {
	return &ProjectRepository{db: db}
}

func (r *ProjectRepository) List(workspaceRecID, projectType string) ([]PackProject, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	projectType = strings.TrimSpace(strings.ToLower(projectType))
	q := `
		SELECT rec_id, workspace_rec_id, project_name, project_version, project_type,
			repo_url, repo_branch, work_dir, config_json, git_env_rec_id, jdk_env_rec_id,
			maven_env_rec_id, nodejs_env_rec_id, deploy_env_rec_id, is_enabled, sort_num, remark,
			created_at, updated_at
		FROM AP_PACK_PROJECT WHERE 1=1`
	var args []any
	if workspaceRecID != "" {
		q += ` AND workspace_rec_id = ?`
		args = append(args, workspaceRecID)
	}
	if projectType != "" {
		q += ` AND LOWER(project_type) = ?`
		args = append(args, projectType)
	}
	q += ` ORDER BY sort_num ASC, project_name ASC, project_version ASC`

	rows, err := r.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]PackProject, 0)
	for rows.Next() {
		item, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, *item)
	}
	return list, rows.Err()
}

func (r *ProjectRepository) FindByRecID(recID string) (*PackProject, error) {
	row := r.db.QueryRow(`
		SELECT rec_id, workspace_rec_id, project_name, project_version, project_type,
			repo_url, repo_branch, work_dir, config_json, git_env_rec_id, jdk_env_rec_id,
			maven_env_rec_id, nodejs_env_rec_id, deploy_env_rec_id, is_enabled, sort_num, remark,
			created_at, updated_at
		FROM AP_PACK_PROJECT WHERE rec_id = ?`, recID)
	return scanProjectRow(row)
}

func (r *ProjectRepository) Insert(item *PackProject) error {
	_, err := r.db.Exec(`
		INSERT INTO AP_PACK_PROJECT(
			rec_id, workspace_rec_id, project_name, project_version, project_type,
			repo_url, repo_branch, work_dir, config_json, git_env_rec_id, jdk_env_rec_id,
			maven_env_rec_id, nodejs_env_rec_id, deploy_env_rec_id, is_enabled, sort_num, remark)
		VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.RecID, item.WorkspaceRecID, item.ProjectName, item.ProjectVersion,
		item.ProjectType, item.RepoURL, item.RepoBranch, item.WorkDir, item.ConfigJSON,
		item.GitEnvRecID, item.JdkEnvRecID, item.MavenEnvRecID, item.NodejsEnvRecID,
		item.DeployEnvRecID, item.IsEnabled, item.SortNum, item.Remark,
	)
	return mapProjectUniqueErr(err)
}

func (r *ProjectRepository) Update(item *PackProject) error {
	res, err := r.db.Exec(`
		UPDATE AP_PACK_PROJECT SET
			workspace_rec_id = ?, project_name = ?, project_version = ?,
			project_type = ?, repo_url = ?, repo_branch = ?, work_dir = ?, config_json = ?,
			git_env_rec_id = ?, jdk_env_rec_id = ?, maven_env_rec_id = ?, nodejs_env_rec_id = ?,
			deploy_env_rec_id = ?, is_enabled = ?, sort_num = ?, remark = ?, updated_at = ?
		WHERE rec_id = ?`,
		item.WorkspaceRecID, item.ProjectName, item.ProjectVersion,
		item.ProjectType, item.RepoURL, item.RepoBranch, item.WorkDir, item.ConfigJSON,
		item.GitEnvRecID, item.JdkEnvRecID, item.MavenEnvRecID, item.NodejsEnvRecID,
		item.DeployEnvRecID, item.IsEnabled, item.SortNum, item.Remark, nowTS(), item.RecID,
	)
	if err != nil {
		return mapProjectUniqueErr(err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrProjectNotFound
	}
	return nil
}

func (r *ProjectRepository) Delete(recID string) error {
	res, err := r.db.Exec(`DELETE FROM AP_PACK_PROJECT WHERE rec_id = ?`, recID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrProjectNotFound
	}
	return nil
}

func mapProjectUniqueErr(err error) error {
	if err == nil {
		return nil
	}
	lower := strings.ToLower(err.Error())
	if strings.Contains(lower, "ix_ap_pack_proj_ws_ver") ||
		strings.Contains(lower, "workspace_rec_id") && strings.Contains(lower, "project_version") ||
		strings.Contains(lower, "unique") {
		return ErrProjectVersionExists
	}
	return err
}

type projectScanner interface {
	Scan(dest ...any) error
}

func scanProject(sc projectScanner) (*PackProject, error) {
	var item PackProject
	var created, updated string
	err := sc.Scan(
		&item.RecID, &item.WorkspaceRecID, &item.ProjectName, &item.ProjectVersion,
		&item.ProjectType, &item.RepoURL, &item.RepoBranch, &item.WorkDir, &item.ConfigJSON,
		&item.GitEnvRecID, &item.JdkEnvRecID, &item.MavenEnvRecID, &item.NodejsEnvRecID,
		&item.DeployEnvRecID, &item.IsEnabled, &item.SortNum, &item.Remark,
		&created, &updated,
	)
	if err != nil {
		return nil, err
	}
	item.CreatedAt = parseTime(created)
	item.UpdatedAt = parseTime(updated)
	return &item, nil
}

func scanProjectRow(row *sql.Row) (*PackProject, error) {
	item, err := scanProject(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, err
	}
	return item, nil
}
