package workspacebundle

import (
	"testing"

	"auto-pack/backend/internal/repository"
)

func TestExportImportRoundTrip(t *testing.T) {
	ws := &repository.PackWorkspace{
		RecID:         "ws-1",
		WorkspaceCode: "demo",
		WorkspaceName: "Demo",
		RootDir:       "",
		IsEnabled:     1,
		SortNum:       1,
		Remark:        "note",
	}
	projects := []repository.PackProject{{
		RecID:          "p-1",
		WorkspaceRecID: "ws-1",
		ProjectName:    "app",
		ProjectVersion: "V1",
		ProjectType:    "maven",
		RepoURL:        "http://example.com/a.git",
		RepoBranch:     "main",
		WorkDir:        "app-V1",
		ConfigJSON:     `{"build_triggers":{"depends_on":[]}}`,
		IsEnabled:      1,
	}}
	bundle := Export(ws, projects)
	if err := bundle.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	dest, err := bundle.BuildWorkspace(NewWorkspaceOptions{
		WorkspaceCode: "demo-copy",
		WorkspaceName: "Demo Copy",
	})
	if err != nil {
		t.Fatalf("BuildWorkspace: %v", err)
	}
	out, err := bundle.BuildProjects(dest.RecID)
	if err != nil {
		t.Fatalf("BuildProjects: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("projects: %d", len(out))
	}
	if out[0].RecID == "p-1" {
		t.Fatal("expected remapped project rec_id")
	}
	if out[0].WorkspaceRecID != dest.RecID {
		t.Fatalf("workspace_rec_id=%q", out[0].WorkspaceRecID)
	}
}
