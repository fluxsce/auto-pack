package projectcfg

import (
	"errors"
	"strings"
)

// BuildStepsBulkOpts selects build-step fields to overwrite in each project's config_json.
type BuildStepsBulkOpts struct {
	UpdateBuildCmd        bool
	BuildCmd              string
	UpdateOutputDir       bool
	OutputDir             string
	UpdateArtifactPattern bool
	ArtifactPattern       string
	UpdateLogRetention    bool
	LogRetentionDays      int
}

// ValidateBuildStepsBulk checks bulk build-step options.
func ValidateBuildStepsBulk(opts BuildStepsBulkOpts) error {
	if !opts.UpdateBuildCmd && !opts.UpdateOutputDir && !opts.UpdateArtifactPattern && !opts.UpdateLogRetention {
		return errors.New("at least one build step field must be selected for update")
	}
	return nil
}

// ApplyBuildStepsToConfig overwrites selected build-step fields in cfg.
func ApplyBuildStepsToConfig(cfg *Config, opts BuildStepsBulkOpts) {
	if opts.UpdateBuildCmd {
		cfg.BuildCmd = strings.TrimSpace(opts.BuildCmd)
	}
	if opts.UpdateOutputDir {
		cfg.OutputDir = strings.TrimSpace(opts.OutputDir)
	}
	if opts.UpdateArtifactPattern {
		cfg.ArtifactPattern = strings.TrimSpace(opts.ArtifactPattern)
	}
	if opts.UpdateLogRetention {
		cfg.LogRetentionDays = EffectiveLogRetentionDays(opts.LogRetentionDays)
	}
}
