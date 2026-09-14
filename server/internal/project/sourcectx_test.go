package project_test

import (
	"path/filepath"
	"testing"

	"auto-pack/server/internal/project"
)

func TestResolveLocalSourcePathExpandsVars(t *testing.T) {
	r := project.ParseRecipe(`{"scm_mode":"local","local_source_path":"/home/appinstall/${AP_WORKSPACE_CODE}/${AP_PROJECT_NAME}"}`)
	ctx := project.SourceContext{
		WorkspaceCode: "P05",
		ProjectName:   "demo-app",
	}
	got := project.ResolveLocalSourcePath(r, ctx)
	want := "/home/appinstall/P05/demo-app"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	abs := project.AbsLocalSourceCtx(r, ctx)
	if abs != filepath.FromSlash(want) {
		t.Fatalf("abs=%q want %q", abs, want)
	}
}

func TestWorkRelCtxExpandsRelativeLocal(t *testing.T) {
	r := project.ParseRecipe(`{"scm_mode":"local","local_source_path":"sources/${AP_PROJECT_NAME}"}`)
	ctx := project.SourceContext{ProjectName: "hub-api"}
	got := project.WorkRelCtx("workspace-app", r, ctx)
	want := "workspace-app/sources/hub-api"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
