package workspacebundle

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectcfg"

	"github.com/google/uuid"
)

const FormatVersion = 1

var (
	ErrUnsupportedFormat = errors.New("unsupported workspace bundle format version")
	ErrBundleEmpty       = errors.New("workspace bundle has no projects")
	ErrWorkspaceRequired = errors.New("workspace bundle is missing workspace metadata")
)

// Bundle is a portable export of one workspace and its project definitions.
type Bundle struct {
	FormatVersion        int             `json:"format_version"`
	ExportedAt           string          `json:"exported_at"`
	SourceWorkspaceRecID string          `json:"source_workspace_rec_id,omitempty"`
	Workspace            WorkspaceExport `json:"workspace"`
	Projects             []ProjectExport `json:"projects"`
}

type WorkspaceExport struct {
	WorkspaceCode string `json:"workspace_code"`
	WorkspaceName string `json:"workspace_name"`
	RootDir       string `json:"root_dir"`
	IsEnabled     int    `json:"is_enabled"`
	SortNum       int    `json:"sort_num"`
	Remark        string `json:"remark"`
}

type ProjectExport struct {
	SourceRecID    string `json:"source_rec_id,omitempty"`
	ProjectName    string `json:"project_name"`
	ProjectVersion string `json:"project_version"`
	ProjectType    string `json:"project_type"`
	RepoURL        string `json:"repo_url"`
	RepoBranch     string `json:"repo_branch"`
	WorkDir        string `json:"work_dir"`
	ConfigJSON     string `json:"config_json"`
	GitEnvRecID    string `json:"git_env_rec_id"`
	JdkEnvRecID    string `json:"jdk_env_rec_id"`
	MavenEnvRecID  string `json:"maven_env_rec_id"`
	NodejsEnvRecID string `json:"nodejs_env_rec_id"`
	DeployEnvRecID string `json:"deploy_env_rec_id"`
	IsEnabled      int    `json:"is_enabled"`
	SortNum        int    `json:"sort_num"`
	Remark         string `json:"remark"`
}

// Export builds a bundle from a workspace and its projects.
func Export(ws *repository.PackWorkspace, projects []repository.PackProject) Bundle {
	out := Bundle{
		FormatVersion:        FormatVersion,
		ExportedAt:           time.Now().UTC().Format(time.RFC3339),
		SourceWorkspaceRecID: strings.TrimSpace(ws.RecID),
		Workspace: WorkspaceExport{
			WorkspaceCode: strings.TrimSpace(ws.WorkspaceCode),
			WorkspaceName: strings.TrimSpace(ws.WorkspaceName),
			RootDir:       strings.TrimSpace(ws.RootDir),
			IsEnabled:     ws.IsEnabled,
			SortNum:       ws.SortNum,
			Remark:        strings.TrimSpace(ws.Remark),
		},
		Projects: make([]ProjectExport, 0, len(projects)),
	}
	for _, p := range projects {
		out.Projects = append(out.Projects, projectToExport(p))
	}
	return out
}

func projectToExport(p repository.PackProject) ProjectExport {
	name, version, repoURL, branch, remark := projectcfg.NormalizeProjectSCM(
		p.ProjectName, p.ProjectVersion, p.RepoURL, p.RepoBranch, p.Remark,
	)
	workDir := strings.TrimSpace(p.WorkDir)
	if workDir == "" {
		workDir = projectcfg.SuggestWorkDir(name)
	}
	return ProjectExport{
		SourceRecID:    strings.TrimSpace(p.RecID),
		ProjectName:    name,
		ProjectVersion: version,
		ProjectType:    strings.TrimSpace(p.ProjectType),
		RepoURL:        repoURL,
		RepoBranch:     branch,
		WorkDir:        workDir,
		ConfigJSON:     strings.TrimSpace(p.ConfigJSON),
		GitEnvRecID:    strings.TrimSpace(p.GitEnvRecID),
		JdkEnvRecID:    strings.TrimSpace(p.JdkEnvRecID),
		MavenEnvRecID:  strings.TrimSpace(p.MavenEnvRecID),
		NodejsEnvRecID: strings.TrimSpace(p.NodejsEnvRecID),
		DeployEnvRecID: strings.TrimSpace(p.DeployEnvRecID),
		IsEnabled:      p.IsEnabled,
		SortNum:        p.SortNum,
		Remark:         remark,
	}
}

// Validate checks bundle structure before import.
func (b *Bundle) Validate() error {
	if b == nil {
		return errors.New("bundle is required")
	}
	if b.FormatVersion != FormatVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedFormat, b.FormatVersion)
	}
	name := strings.TrimSpace(b.Workspace.WorkspaceName)
	if name == "" {
		return ErrWorkspaceRequired
	}
	if err := projectcfg.ValidateWorkspaceCode(b.Workspace.WorkspaceCode); err != nil {
		return err
	}
	if len(b.Projects) == 0 {
		return ErrBundleEmpty
	}
	for i, p := range b.Projects {
		if err := validateProjectExport(&p); err != nil {
			return fmt.Errorf("project[%d]: %w", i, err)
		}
	}
	return nil
}

func validateProjectExport(p *ProjectExport) error {
	name, version, _, _, _ := projectcfg.NormalizeProjectSCM(
		p.ProjectName, p.ProjectVersion, p.RepoURL, p.RepoBranch, p.Remark,
	)
	if err := projectcfg.ValidateProjectName(name); err != nil {
		return err
	}
	if err := projectcfg.ValidateVersion(version); err != nil {
		return err
	}
	if strings.TrimSpace(p.ProjectType) == "" {
		return errors.New("project_type is required")
	}
	cfg, err := projectcfg.ParseConfigJSON(p.ConfigJSON)
	if err != nil {
		return err
	}
	if err := projectcfg.ValidateBuildTriggers("", cfg.BuildTriggers); err != nil {
		return err
	}
	return nil
}

// NewWorkspaceOptions overrides bundle workspace fields when importing as a new workspace.
type NewWorkspaceOptions struct {
	WorkspaceCode string
	WorkspaceName string
	RootDir       string
	IsEnabled     *int
	SortNum       *int
	Remark        string
}

// BuildWorkspace creates a new workspace row for import.
func (b *Bundle) BuildWorkspace(opts NewWorkspaceOptions) (*repository.PackWorkspace, error) {
	if err := b.Validate(); err != nil {
		return nil, err
	}
	code := strings.TrimSpace(opts.WorkspaceCode)
	if code == "" {
		code = strings.TrimSpace(b.Workspace.WorkspaceCode)
	}
	name := strings.TrimSpace(opts.WorkspaceName)
	if name == "" {
		name = strings.TrimSpace(b.Workspace.WorkspaceName)
	}
	if name == "" {
		return nil, errors.New("workspace_name is required")
	}
	if err := projectcfg.ValidateWorkspaceCode(code); err != nil {
		return nil, err
	}
	rootDir := strings.TrimSpace(opts.RootDir)
	if rootDir == "" {
		rootDir = strings.TrimSpace(b.Workspace.RootDir)
	}
	isEnabled := b.Workspace.IsEnabled
	if opts.IsEnabled != nil {
		isEnabled = *opts.IsEnabled
	}
	if isEnabled != 0 {
		isEnabled = 1
	}
	sortNum := b.Workspace.SortNum
	if opts.SortNum != nil {
		sortNum = *opts.SortNum
	}
	remark := strings.TrimSpace(opts.Remark)
	if remark == "" {
		remark = strings.TrimSpace(b.Workspace.Remark)
	}
	return &repository.PackWorkspace{
		RecID:         uuid.New().String(),
		WorkspaceCode: code,
		WorkspaceName: name,
		RootDir:       rootDir,
		IsDefault:     0,
		IsEnabled:     isEnabled,
		SortNum:       sortNum,
		Remark:        remark,
	}, nil
}

// BuildProjects materializes project rows for a target workspace, remapping depends_on IDs.
func (b *Bundle) BuildProjects(workspaceRecID string) ([]repository.PackProject, error) {
	workspaceRecID = strings.TrimSpace(workspaceRecID)
	if workspaceRecID == "" {
		return nil, errors.New("workspace_rec_id is required")
	}
	if err := b.Validate(); err != nil {
		return nil, err
	}
	idMap := make(map[string]string, len(b.Projects))
	for _, p := range b.Projects {
		src := strings.TrimSpace(p.SourceRecID)
		if src != "" {
			idMap[src] = uuid.New().String()
		}
	}
	out := make([]repository.PackProject, 0, len(b.Projects))
	for _, src := range b.Projects {
		newID := idMap[strings.TrimSpace(src.SourceRecID)]
		if newID == "" {
			newID = uuid.New().String()
		}
		item, err := b.projectToModel(workspaceRecID, newID, src, idMap)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (b *Bundle) projectToModel(workspaceRecID, recID string, src ProjectExport, idMap map[string]string) (repository.PackProject, error) {
	name, version, repoURL, branch, remark := projectcfg.NormalizeProjectSCM(
		src.ProjectName, src.ProjectVersion, src.RepoURL, src.RepoBranch, src.Remark,
	)
	cfgJSON, err := projectcfg.RemapBuildDependsOn(src.ConfigJSON, idMap)
	if err != nil {
		return repository.PackProject{}, err
	}
	normalized, err := projectcfg.NormalizeConfigJSON(cfgJSON)
	if err != nil {
		return repository.PackProject{}, err
	}
	cfg, err := projectcfg.ParseConfigJSON(normalized)
	if err != nil {
		return repository.PackProject{}, err
	}
	if err := projectcfg.ValidateBuildTriggers(recID, cfg.BuildTriggers); err != nil {
		return repository.PackProject{}, err
	}
	workDir := strings.TrimSpace(src.WorkDir)
	if workDir == "" {
		workDir = projectcfg.SuggestWorkDir(name)
	}
	isEnabled := src.IsEnabled
	if isEnabled != 0 {
		isEnabled = 1
	}
	return repository.PackProject{
		RecID:          recID,
		WorkspaceRecID: workspaceRecID,
		ProjectName:    name,
		ProjectVersion: version,
		ProjectType:    strings.TrimSpace(src.ProjectType),
		RepoURL:        repoURL,
		RepoBranch:     branch,
		WorkDir:        workDir,
		ConfigJSON:     normalized,
		GitEnvRecID:    strings.TrimSpace(src.GitEnvRecID),
		JdkEnvRecID:    strings.TrimSpace(src.JdkEnvRecID),
		MavenEnvRecID:  strings.TrimSpace(src.MavenEnvRecID),
		NodejsEnvRecID: strings.TrimSpace(src.NodejsEnvRecID),
		DeployEnvRecID: strings.TrimSpace(src.DeployEnvRecID),
		IsEnabled:      isEnabled,
		SortNum:        src.SortNum,
		Remark:         remark,
	}, nil
}
