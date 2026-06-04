package projectcfg

import "testing"

func TestValidateBuildTriggers_selfDependency(t *testing.T) {
	t.Parallel()
	err := ValidateBuildTriggers("p1", BuildTriggers{DependsOn: []string{"p1"}})
	if err == nil {
		t.Fatal("expected error for self dependency")
	}
}

func TestValidateBuildTriggers_cron(t *testing.T) {
	t.Parallel()
	err := ValidateBuildTriggers("", BuildTriggers{
		Schedule: BuildSchedule{Enabled: true, Cron: "not a cron"},
	})
	if err == nil {
		t.Fatal("expected invalid cron error")
	}
	if err := ValidateBuildTriggers("", BuildTriggers{
		Schedule: BuildSchedule{Enabled: true, Cron: "0 2 * * *"},
	}); err != nil {
		t.Fatal(err)
	}
}
