package projectcfg

import (
	"encoding/json"
	"errors"
	"strings"
)

// Config holds build-related fields inside config_json.
type Config struct {
	ScmMode          string            `json:"scm_mode,omitempty"`
	LocalSourcePath  string            `json:"local_source_path,omitempty"`
	BuildCmd         string            `json:"build_cmd"`
	OutputDir        string            `json:"output_dir"`
	ArtifactPattern  string            `json:"artifact_pattern"`
	LogRetentionDays int               `json:"log_retention_days"`
	PostBuildActions []PostBuildAction `json:"post_build_actions"`
	BuildTriggers    BuildTriggers     `json:"build_triggers"`
}

func ParseConfigJSON(raw string) (Config, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Config{}, nil
	}
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return Config{}, errors.New("invalid config_json")
	}
	if cfg.BuildTriggers.DependsOn == nil {
		cfg.BuildTriggers.DependsOn = []string{}
	}
	cfg.LogRetentionDays = EffectiveLogRetentionDays(cfg.LogRetentionDays)
	return cfg, nil
}

// MarshalConfigJSON serializes config for storage in config_json.
func MarshalConfigJSON(cfg Config) (string, error) {
	cfg.LogRetentionDays = EffectiveLogRetentionDays(cfg.LogRetentionDays)
	if cfg.BuildTriggers.DependsOn == nil {
		cfg.BuildTriggers.DependsOn = []string{}
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func NormalizeConfigJSON(raw string) (string, error) {
	cfg, err := ParseConfigJSON(raw)
	if err != nil {
		return "", err
	}
	cfg.LogRetentionDays = EffectiveLogRetentionDays(cfg.LogRetentionDays)
	if cfg.ScmMode == "" && cfg.LocalSourcePath == "" &&
		cfg.BuildCmd == "" && cfg.OutputDir == "" && cfg.ArtifactPattern == "" &&
		len(cfg.PostBuildActions) == 0 && len(cfg.BuildTriggers.DependsOn) == 0 &&
		!cfg.BuildTriggers.Schedule.Enabled &&
		cfg.LogRetentionDays == DefaultLogRetentionDays {
		return "{}", nil
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		return "{}", nil
	}
	return string(b), nil
}
