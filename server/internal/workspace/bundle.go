package workspace

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	// FormatV1 是试用版导出（config_json + 工程级环境 ID）。
	FormatV1 = 1
	// FormatV2 是当前导出（recipe_json + cron_expr）。
	FormatV2 = 2
)

var (
	// ErrUnsupportedFormat 导出文件版本无法识别。
	ErrUnsupportedFormat = errors.New("unsupported workspace bundle format")
	// ErrBundleEmpty 缺少空间元数据。
	ErrBundleEmpty = errors.New("workspace bundle is empty")
	// ErrImportMode 导入 mode 只能是 new 或 replace。
	ErrImportMode = errors.New("import mode must be new or replace")
)

// Bundle 是可移植的空间工程定义，不含环境密文、成员与作业。
type Bundle struct {
	FormatVersion        int             `json:"format_version"`
	ExportedAt           string          `json:"exported_at,omitempty"`
	SourceWorkspaceRecID string          `json:"source_workspace_rec_id,omitempty"`
	Workspace            BundleWorkspace `json:"workspace"`
	Projects             []BundleProject `json:"projects"`
}

// BundleWorkspace 是导出里的空间元数据。
type BundleWorkspace struct {
	WorkspaceCode string `json:"workspace_code"`
	WorkspaceName string `json:"workspace_name"`
	Remark        string `json:"remark,omitempty"`
}

// BundleProject 是一条工程定义。v1 填 config_json；v2 填 recipe_json。
type BundleProject struct {
	SourceRecID    string `json:"source_rec_id,omitempty"`
	ProjectName    string `json:"project_name"`
	ProjectType    string `json:"project_type"`
	ProjectVersion string `json:"project_version,omitempty"`
	RepoURL        string `json:"repo_url,omitempty"`
	RepoBranch     string `json:"repo_branch,omitempty"`
	WorkDir        string `json:"work_dir,omitempty"`
	WorkRelPath    string `json:"work_rel_path,omitempty"`
	RecipeJSON     string `json:"recipe_json,omitempty"`
	ConfigJSON     string `json:"config_json,omitempty"`
	CronExpr       string `json:"cron_expr,omitempty"`
	GitEnvRecID    string `json:"git_env_rec_id,omitempty"`
	JDKEnvRecID    string `json:"jdk_env_rec_id,omitempty"`
	MavenEnvRecID  string `json:"maven_env_rec_id,omitempty"`
	NodeEnvRecID   string `json:"node_env_rec_id,omitempty"`
	NodeJSEnvRecID string `json:"nodejs_env_rec_id,omitempty"`
	DeployEnvRecID string `json:"deploy_env_rec_id,omitempty"`
	IsEnabled      int    `json:"is_enabled"`
	Remark         string `json:"remark,omitempty"`
}

// ImportRequest 是导入入参。
type ImportRequest struct {
	Mode              string `json:"mode"`
	TargetWorkspaceID string `json:"target_workspace_rec_id,omitempty"`
	WorkspaceCode     string `json:"workspace_code,omitempty"`
	WorkspaceName     string `json:"workspace_name,omitempty"`
	Bundle            Bundle `json:"bundle"`
}

// ImportResult 是导入结果。
type ImportResult struct {
	Workspace
	ProjectsImported int    `json:"projects_imported"`
	Mode             string `json:"mode"`
	FormatVersion    int    `json:"format_version"`
}

// Validate 检查包头与空间编码。
func (b Bundle) Validate() error {
	if b.FormatVersion != 0 && b.FormatVersion != FormatV1 && b.FormatVersion != FormatV2 {
		return ErrUnsupportedFormat
	}
	if strings.TrimSpace(b.Workspace.WorkspaceCode) == "" && strings.TrimSpace(b.Workspace.WorkspaceName) == "" {
		return ErrBundleEmpty
	}
	return nil
}

func (b Bundle) resolvedFormat() int {
	if b.FormatVersion == FormatV1 || b.FormatVersion == FormatV2 {
		return b.FormatVersion
	}
	for _, p := range b.Projects {
		if strings.TrimSpace(p.ConfigJSON) != "" && strings.TrimSpace(p.RecipeJSON) == "" {
			return FormatV1
		}
	}
	return FormatV2
}

func stampBundle(b Bundle) Bundle {
	b.FormatVersion = FormatV2
	b.ExportedAt = time.Now().UTC().Format(time.RFC3339)
	return b
}

func marshalRecipe(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
