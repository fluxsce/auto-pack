package release

import (
	"context"
	"testing"

	"auto-pack/server/internal/project"
	"auto-pack/server/internal/release/record"
)

func TestReleaserSkipsUnsupportedType(t *testing.T) {
	out := (&Releaser{}).Run(context.Background(), project.Project{ProjectName: "cli", ProjectType: "go"}, Request{
		WorkDir: t.TempDir(), ReleaseBranch: "release/V1", Tag: "V1",
	})
	if out.Status != record.ResultSkipped {
		t.Fatalf("%+v", out)
	}
}

func TestReleaserRejectsEmptyWorkDir(t *testing.T) {
	out := (&Releaser{}).Run(context.Background(), project.Project{ProjectName: "app", ProjectType: "maven"}, Request{
		ReleaseBranch: "release/V1", Tag: "V1",
	})
	if out.Status != record.ResultFailed || out.Message == "" {
		t.Fatalf("%+v", out)
	}
}
