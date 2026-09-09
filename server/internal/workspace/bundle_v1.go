package workspace

import (
	"encoding/json"
	"path/filepath"
	"strings"
)

// recipeDTO 与 project.Recipe JSON 对齐，避免 workspace↔project 循环引用。
type recipeDTO struct {
	Command          string    `json:"command"`
	Workdir          string    `json:"workdir,omitempty"`
	AutoSync         *bool     `json:"auto_sync,omitempty"`
	SCMMode          string    `json:"scm_mode,omitempty"`
	LocalSourcePath  string    `json:"local_source_path,omitempty"`
	OutputDir        string    `json:"output_dir,omitempty"`
	ArtifactPattern  string    `json:"artifact_pattern,omitempty"`
	GitEnvRecID      string    `json:"git_env_rec_id,omitempty"`
	JDKEnvRecID      string    `json:"jdk_env_rec_id,omitempty"`
	MavenEnvRecID    string    `json:"maven_env_rec_id,omitempty"`
	NodeEnvRecID     string    `json:"node_env_rec_id,omitempty"`
	LogRetentionDays int       `json:"log_retention_days,omitempty"`
	DependsOn        []string  `json:"depends_on,omitempty"`
	PostActions      []postDTO `json:"post_actions,omitempty"`
}

type postDTO struct {
	ID         string `json:"id,omitempty"`
	Kind       string `json:"kind"`
	Enabled    *bool  `json:"enabled,omitempty"`
	EnvRecID   string `json:"env_rec_id,omitempty"`
	RemotePath string `json:"remote_path,omitempty"`
	Command    string `json:"command,omitempty"`
}

type v1Config struct {
	SCMMode          string     `json:"scm_mode"`
	LocalSourcePath  string     `json:"local_source_path"`
	BuildCmd         string     `json:"build_cmd"`
	OutputDir        string     `json:"output_dir"`
	ArtifactPattern  string     `json:"artifact_pattern"`
	LogRetentionDays int        `json:"log_retention_days"`
	PostBuildActions []v1Post   `json:"post_build_actions"`
	BuildTriggers    v1Triggers `json:"build_triggers"`
}

type v1Triggers struct {
	DependsOn []string `json:"depends_on"`
	Schedule  v1Sched  `json:"schedule"`
}

type v1Sched struct {
	Enabled bool   `json:"enabled"`
	Cron    string `json:"cron"`
}

type v1Post struct {
	ID           string       `json:"id"`
	Type         string       `json:"type"`
	Enabled      *bool        `json:"enabled"`
	EnvRecID     string       `json:"env_rec_id"`
	Command      string       `json:"command"`
	TransferSets []v1Transfer `json:"transfer_sets"`
}

type v1Transfer struct {
	RemoteDir string `json:"remote_dir"`
}

func (p BundleProject) toRecipe() (recipeJSON, cronExpr, workRel string) {
	if strings.TrimSpace(p.RecipeJSON) != "" {
		var r recipeDTO
		_ = json.Unmarshal([]byte(p.RecipeJSON), &r)
		workRel = firstNonEmpty(strings.TrimSpace(p.WorkRelPath), strings.TrimSpace(p.WorkDir), recipeWorkRel(p.ProjectName, r))
		return p.RecipeJSON, strings.TrimSpace(p.CronExpr), workRel
	}
	r, cron := convertV1Project(p)
	workRel = firstNonEmpty(strings.TrimSpace(p.WorkRelPath), strings.TrimSpace(p.WorkDir), recipeWorkRel(p.ProjectName, r))
	return marshalRecipe(r), cron, workRel
}

func convertV1Project(p BundleProject) (recipeDTO, string) {
	var cfg v1Config
	_ = json.Unmarshal([]byte(p.ConfigJSON), &cfg)
	autoSync := true
	r := recipeDTO{
		Command:          strings.TrimSpace(cfg.BuildCmd),
		SCMMode:          strings.TrimSpace(cfg.SCMMode),
		LocalSourcePath:  strings.TrimSpace(cfg.LocalSourcePath),
		OutputDir:        strings.TrimSpace(cfg.OutputDir),
		ArtifactPattern:  strings.TrimSpace(cfg.ArtifactPattern),
		LogRetentionDays: cfg.LogRetentionDays,
		AutoSync:         &autoSync,
		GitEnvRecID:      strings.TrimSpace(p.GitEnvRecID),
		JDKEnvRecID:      strings.TrimSpace(p.JDKEnvRecID),
		MavenEnvRecID:    strings.TrimSpace(p.MavenEnvRecID),
		NodeEnvRecID:     firstNonEmpty(strings.TrimSpace(p.NodeEnvRecID), strings.TrimSpace(p.NodeJSEnvRecID)),
		DependsOn:        cleanIDs(cfg.BuildTriggers.DependsOn),
		PostActions:      convertV1Posts(cfg.PostBuildActions, strings.TrimSpace(p.DeployEnvRecID)),
	}
	cron := ""
	if cfg.BuildTriggers.Schedule.Enabled {
		cron = strings.TrimSpace(cfg.BuildTriggers.Schedule.Cron)
	}
	if cron == "" {
		cron = strings.TrimSpace(p.CronExpr)
	}
	return r, cron
}

func convertV1Posts(items []v1Post, deployEnv string) []postDTO {
	out := make([]postDTO, 0, len(items))
	for _, item := range items {
		kind, remote, command := mapV1Post(item)
		if kind == "" {
			continue
		}
		out = append(out, postDTO{
			ID:         strings.TrimSpace(item.ID),
			Kind:       kind,
			Enabled:    item.Enabled,
			EnvRecID:   strings.TrimSpace(item.EnvRecID),
			RemotePath: remote,
			Command:    command,
		})
	}
	if len(out) == 0 && deployEnv != "" {
		out = append(out, postDTO{Kind: "ftp", EnvRecID: deployEnv})
	}
	return out
}

func mapV1Post(item v1Post) (kind, remote, command string) {
	command = strings.TrimSpace(item.Command)
	if len(item.TransferSets) > 0 {
		remote = strings.TrimSpace(item.TransferSets[0].RemoteDir)
	}
	switch strings.ToLower(strings.TrimSpace(item.Type)) {
	case "shell_exec", "shell":
		return "shell", "", command
	case "ssh_exec":
		return "ssh", remote, command
	case "ssh_transfer", "ssh":
		return "ssh", remote, command
	case "email_notify", "email":
		return "email", "", command
	case "ftp_transfer", "ftp":
		return "ftp", remote, command
	case "sftp_transfer", "sftp":
		return "sftp", remote, command
	default:
		return "", "", ""
	}
}

func recipeWorkRel(name string, r recipeDTO) string {
	name = strings.TrimSpace(name)
	extra := filepath.ToSlash(strings.TrimSpace(r.LocalSourcePath))
	extra = strings.Trim(extra, "/")
	if strings.EqualFold(strings.TrimSpace(r.SCMMode), "local") && extra != "" {
		if name == "" {
			return extra
		}
		return name + "/" + extra
	}
	return name
}

func remapRecipeDepends(raw string, idMap map[string]string) string {
	var r recipeDTO
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return raw
	}
	if len(r.DependsOn) == 0 {
		return raw
	}
	next := make([]string, 0, len(r.DependsOn))
	for _, id := range r.DependsOn {
		id = strings.TrimSpace(id)
		if mapped, ok := idMap[id]; ok && mapped != "" {
			next = append(next, mapped)
		}
	}
	r.DependsOn = next
	return marshalRecipe(r)
}

func cleanIDs(ids []string) []string {
	var out []string
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
