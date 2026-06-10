package projectcfg

import "testing"

func TestApplyBuildStepsToConfig_partial(t *testing.T) {
	cfg := Config{
		BuildCmd:         "old cmd",
		OutputDir:        "old/out",
		ArtifactPattern:  "old.*",
		LogRetentionDays: 7,
	}
	ApplyBuildStepsToConfig(&cfg, BuildStepsBulkOpts{
		UpdateBuildCmd: true,
		BuildCmd:       "  mvn package  ",
	})
	if cfg.BuildCmd != "mvn package" {
		t.Fatalf("BuildCmd = %q, want mvn package", cfg.BuildCmd)
	}
	if cfg.OutputDir != "old/out" {
		t.Fatalf("OutputDir changed unexpectedly: %q", cfg.OutputDir)
	}

	ApplyBuildStepsToConfig(&cfg, BuildStepsBulkOpts{
		UpdateLogRetention: true,
		LogRetentionDays:   0,
	})
	if cfg.LogRetentionDays != DefaultLogRetentionDays {
		t.Fatalf("LogRetentionDays = %d, want default %d", cfg.LogRetentionDays, DefaultLogRetentionDays)
	}
}

func TestValidateBuildStepsBulk_requiresField(t *testing.T) {
	if err := ValidateBuildStepsBulk(BuildStepsBulkOpts{}); err == nil {
		t.Fatal("expected error when no fields selected")
	}
}
