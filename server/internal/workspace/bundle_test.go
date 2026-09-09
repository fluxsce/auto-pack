package workspace_test

import (
	"encoding/json"
	"errors"
	"testing"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/workspace"
)

type recipeView struct {
	Command     string   `json:"command"`
	SCMMode     string   `json:"scm_mode"`
	GitEnvRecID string   `json:"git_env_rec_id"`
	JDKEnvRecID string   `json:"jdk_env_rec_id"`
	DependsOn   []string `json:"depends_on"`
	PostActions []struct {
		Kind       string `json:"kind"`
		RemotePath string `json:"remote_path"`
	} `json:"post_actions"`
}

func TestConvertV1Bundle(t *testing.T) {
	svc, st, ctx, actor, _ := openWS(t)
	raw := `{
		"format_version": 1,
		"workspace": {"workspace_code": "demo", "workspace_name": "演示"},
		"projects": [{
			"source_rec_id": "p-old",
			"project_name": "app",
			"project_type": "maven",
			"repo_url": "https://example.com/a.git",
			"repo_branch": "main",
			"work_dir": "app",
			"git_env_rec_id": "git-1",
			"jdk_env_rec_id": "jdk-1",
			"config_json": "{\"scm_mode\":\"git\",\"build_cmd\":\"mvn -B package\",\"output_dir\":\"target\",\"artifact_pattern\":\"*.jar\",\"log_retention_days\":7,\"post_build_actions\":[{\"id\":\"a1\",\"type\":\"ssh_transfer\",\"enabled\":true,\"env_rec_id\":\"ssh-1\",\"transfer_sets\":[{\"remote_dir\":\"/opt/app\"}]}],\"build_triggers\":{\"depends_on\":[\"p-dep\"],\"schedule\":{\"enabled\":true,\"cron\":\"0 2 * * *\"}}}",
			"is_enabled": 1
		}, {
			"source_rec_id": "p-dep",
			"project_name": "lib",
			"project_type": "maven",
			"config_json": "{\"build_cmd\":\"mvn -B install\"}",
			"is_enabled": 1
		}]
	}`
	var bundle workspace.Bundle
	if err := json.Unmarshal([]byte(raw), &bundle); err != nil {
		t.Fatal(err)
	}
	out, err := svc.Import(ctx, actor, workspace.ImportRequest{Mode: "new", Bundle: bundle})
	if err != nil {
		t.Fatal(err)
	}
	if out.ProjectsImported != 2 || out.FormatVersion != 1 {
		t.Fatalf("imported=%d format=%d", out.ProjectsImported, out.FormatVersion)
	}
	q := st.Rebind(`SELECT rec_id, recipe_json, COALESCE(cron_expr,''), COALESCE(repo_url,'') FROM AP_PROJECT WHERE workspace_rec_id = ? AND project_name = ?`)
	var appID, recipeRaw, cron, repo string
	if err := st.SQL().QueryRowContext(ctx, q, out.RecID, "app").Scan(&appID, &recipeRaw, &cron, &repo); err != nil {
		t.Fatal(err)
	}
	if cron != "0 2 * * *" || repo != "https://example.com/a.git" {
		t.Fatalf("cron=%q repo=%q", cron, repo)
	}
	var r recipeView
	if err := json.Unmarshal([]byte(recipeRaw), &r); err != nil {
		t.Fatal(err)
	}
	if r.Command != "mvn -B package" || r.SCMMode != "git" || r.GitEnvRecID != "git-1" || r.JDKEnvRecID != "jdk-1" {
		t.Fatalf("recipe %+v", r)
	}
	if len(r.PostActions) != 1 || r.PostActions[0].Kind != "ssh" || r.PostActions[0].RemotePath != "/opt/app" {
		t.Fatalf("post %+v", r.PostActions)
	}
	var depID string
	if err := st.SQL().QueryRowContext(ctx, st.Rebind(`SELECT rec_id FROM AP_PROJECT WHERE workspace_rec_id = ? AND project_name = ?`), out.RecID, "lib").Scan(&depID); err != nil {
		t.Fatal(err)
	}
	if len(r.DependsOn) != 1 || r.DependsOn[0] != depID {
		t.Fatalf("depends_on=%v want %s", r.DependsOn, depID)
	}
}

func TestImportVersionTagAsProjectName(t *testing.T) {
	svc, st, ctx, actor, _ := openWS(t)
	raw := `{
		"format_version": 1,
		"workspace": {"workspace_code": "V9.0.0-P09", "workspace_name": "V9.0.0-P09"},
		"projects": [{
			"project_name": "V9.0.0-P09",
			"project_type": "custom",
			"work_dir": "V9.0.0-P09",
			"config_json": "{\"scm_mode\":\"local\",\"local_source_path\":\"/home/appinstall/V9.0.0-P09\"}",
			"is_enabled": 1
		}]
	}`
	var bundle workspace.Bundle
	if err := json.Unmarshal([]byte(raw), &bundle); err != nil {
		t.Fatal(err)
	}
	out, err := svc.Import(ctx, actor, workspace.ImportRequest{Mode: "new", Bundle: bundle})
	if err != nil {
		t.Fatal(err)
	}
	if out.ProjectsImported != 1 || out.WorkspaceCode != "V9.0.0-P09" {
		t.Fatalf("%+v", out)
	}
	var name, work string
	q := st.Rebind(`SELECT project_name, COALESCE(work_rel_path,'') FROM AP_PROJECT WHERE workspace_rec_id = ?`)
	if err := st.SQL().QueryRowContext(ctx, q, out.RecID).Scan(&name, &work); err != nil {
		t.Fatal(err)
	}
	if name != "V9.0.0-P09" || work != "V9.0.0-P09" {
		t.Fatalf("name=%s work=%s", name, work)
	}
}

func TestExportImportRoundTrip(t *testing.T) {
	svc, st, ctx, actor, _ := openWS(t)
	src, err := svc.Create(ctx, actor, "srcws", "源空间")
	if err != nil {
		t.Fatal(err)
	}
	insertProject(t, st, ctx, src.RecID, "demo")
	bundle, err := svc.Export(ctx, actor, src.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.FormatVersion != workspace.FormatV2 || len(bundle.Projects) != 1 {
		t.Fatalf("bundle %+v", bundle)
	}
	out, err := svc.Import(ctx, actor, workspace.ImportRequest{
		Mode:          "new",
		WorkspaceCode: "copied",
		WorkspaceName: "副本",
		Bundle:        bundle,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.ProjectsImported != 1 || out.WorkspaceCode != "copied" {
		t.Fatalf("%+v", out)
	}
}

func TestImportReplaceProjects(t *testing.T) {
	svc, st, ctx, actor, _ := openWS(t)
	target := access.DefaultWorkspaceRecID
	insertProject(t, st, ctx, target, "old-app")
	bundle := workspace.Bundle{
		FormatVersion: workspace.FormatV2,
		Workspace:     workspace.BundleWorkspace{WorkspaceCode: "x", WorkspaceName: "x"},
		Projects: []workspace.BundleProject{{
			ProjectName: "new-app",
			ProjectType: "npm",
			RecipeJSON:  `{"command":"npm run build"}`,
			IsEnabled:   1,
		}},
	}
	out, err := svc.Import(ctx, actor, workspace.ImportRequest{Mode: "replace", TargetWorkspaceID: target, Bundle: bundle})
	if err != nil {
		t.Fatal(err)
	}
	if out.ProjectsImported != 1 {
		t.Fatalf("imported %d", out.ProjectsImported)
	}
	var n int
	if err := st.SQL().QueryRowContext(ctx, st.Rebind(`SELECT COUNT(1) FROM AP_PROJECT WHERE workspace_rec_id = ?`), target).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("want 1 project, got %d", n)
	}
}

func TestImportDuplicateCode(t *testing.T) {
	svc, ctx, actor, _ := setupWS(t)
	bundle := workspace.Bundle{
		FormatVersion: workspace.FormatV2,
		Workspace:     workspace.BundleWorkspace{WorkspaceCode: "default", WorkspaceName: "默认工作空间"},
		Projects: []workspace.BundleProject{{
			ProjectName: "app",
			ProjectType: "custom",
			RecipeJSON:  `{}`,
			IsEnabled:   1,
		}},
	}
	_, err := svc.Import(ctx, actor, workspace.ImportRequest{Mode: "new", Bundle: bundle})
	if !errors.Is(err, workspace.ErrWorkspaceCodeTaken) {
		t.Fatalf("want code taken, got %v", err)
	}
}

func TestCreateDuplicateCode(t *testing.T) {
	svc, ctx, actor, _ := setupWS(t)
	_, err := svc.Create(ctx, actor, "default", "另一份默认")
	if !errors.Is(err, workspace.ErrWorkspaceCodeTaken) {
		t.Fatalf("want code taken, got %v", err)
	}
}

func TestRefuseUnknownBundle(t *testing.T) {
	svc, ctx, actor, _ := setupWS(t)
	_, err := svc.Import(ctx, actor, workspace.ImportRequest{
		Mode:   "new",
		Bundle: workspace.Bundle{FormatVersion: 9, Workspace: workspace.BundleWorkspace{WorkspaceCode: "a", WorkspaceName: "A"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
