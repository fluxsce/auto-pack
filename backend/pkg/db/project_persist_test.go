package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectcfg"

	_ "modernc.org/sqlite"
)

func TestProjectRepository_PersistsAllFields(t *testing.T) {
	database, err := openTestDBWithMigrations(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	projects := repository.NewProject(database)
	wsID := repository.DefaultWorkspaceRecID

	cfgJSON, err := projectcfg.NormalizeConfigJSON(`{"build_cmd":"mvn package","output_dir":"target","artifact_pattern":"*.jar"}`)
	if err != nil {
		t.Fatal(err)
	}

	item := &repository.PackProject{
		RecID:          "11111111-1111-4111-8111-111111111111",
		WorkspaceRecID: wsID,
		ProjectName:    "Demo App",
		ProjectVersion: "1.0.0",
		ProjectType:    "maven",
		RepoURL:        "https://git.example.com/org/demo.git",
		RepoBranch:     "develop",
		WorkDir:        "demo-app",
		ConfigJSON:     cfgJSON,
		GitEnvRecID:    "22222222-2222-4222-8222-222222222222",
		JdkEnvRecID:    "33333333-3333-4333-8333-333333333333",
		MavenEnvRecID:  "44444444-4444-4444-8444-444444444444",
		NodejsEnvRecID: "",
		DeployEnvRecID: "",
		IsEnabled:      1,
		SortNum:        5,
		Remark:         "integration test",
	}
	if err := projects.Insert(item); err != nil {
		t.Fatal(err)
	}

	got, err := projects.FindByRecID(item.RecID)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(t, "workspace_rec_id", wsID, got.WorkspaceRecID)
	assertEq(t, "project_name", item.ProjectName, got.ProjectName)
	assertEq(t, "project_version", item.ProjectVersion, got.ProjectVersion)
	assertEq(t, "repo_url", item.RepoURL, got.RepoURL)
	assertEq(t, "repo_branch", item.RepoBranch, got.RepoBranch)
	assertEq(t, "work_dir", item.WorkDir, got.WorkDir)
	assertEq(t, "config_json", cfgJSON, got.ConfigJSON)
	assertEq(t, "git_env_rec_id", item.GitEnvRecID, got.GitEnvRecID)
	assertEq(t, "jdk_env_rec_id", item.JdkEnvRecID, got.JdkEnvRecID)
	assertEq(t, "maven_env_rec_id", item.MavenEnvRecID, got.MavenEnvRecID)
	assertEq(t, "remark", item.Remark, got.Remark)

	item.RepoBranch = "v2.0.0"
	item.GitEnvRecID = ""
	item.ConfigJSON = `{}`
	if err := projects.Update(item); err != nil {
		t.Fatal(err)
	}
	updated, err := projects.FindByRecID(item.RecID)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "repo_branch after update", "v2.0.0", updated.RepoBranch)
	assertEq(t, "git_env_rec_id cleared", "", updated.GitEnvRecID)
	assertEq(t, "config_json cleared", `{}`, updated.ConfigJSON)
}

func TestNormalizeProjectWorkDirsWhileRowsOpen(t *testing.T) {
	database, err := openTestDBWithMigrations(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	projects := repository.NewProject(database)
	item := &repository.PackProject{
		RecID:          "22222222-2222-4222-8222-222222222222",
		WorkspaceRecID: repository.DefaultWorkspaceRecID,
		ProjectName:    "Legacy App",
		ProjectVersion: "2.0.0",
		ProjectType:    "custom",
		WorkDir:        "Legacy-App/2.0.0",
		ConfigJSON:     `{}`,
		IsEnabled:      1,
	}
	if err := projects.Insert(item); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	if err := normalizeProjectWorkDirs(ctx, database); err != nil {
		t.Fatal(err)
	}
	if err := normalizeProjectWorkDirs(ctx, database); err != nil {
		t.Fatal(err)
	}

	got, err := projects.FindByRecID(item.RecID)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(t, "work_dir normalized", "Legacy-App", got.WorkDir)
}

func TestEnsurePackProjectWorkspaceColumns(t *testing.T) {
	database, err := sqlOpenMemoryWithLegacyProjectTable(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	if err := ensurePackProjectWorkspaceColumns(context.Background(), database); err != nil {
		t.Fatal(err)
	}

	hasWS, err := columnExists(context.Background(), database, "AP_PACK_PROJECT", "workspace_rec_id")
	if err != nil || !hasWS {
		t.Fatalf("workspace_rec_id column missing: err=%v has=%v", err, hasWS)
	}
	hasVer, err := columnExists(context.Background(), database, "AP_PACK_PROJECT", "project_version")
	if err != nil || !hasVer {
		t.Fatalf("project_version column missing: err=%v has=%v", err, hasVer)
	}
}

func sqlOpenMemoryWithLegacyProjectTable(t *testing.T) (*sql.DB, error) {
	t.Helper()
	path := filepath.Join("..", "..", "..", "scripts", "sqlite", "003_pack_project.sql")
	script, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	if err := applyScript(context.Background(), database, script); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

func openTestDBWithMigrations(t *testing.T) (*sql.DB, error) {
	t.Helper()
	dir := filepath.Join("..", "..", "..", "scripts", "sqlite")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	ctx := context.Background()
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		script, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			database.Close()
			return nil, err
		}
		if err := applyScript(ctx, database, script); err != nil {
			database.Close()
			return nil, err
		}
	}
	if err := ensurePackProjectWorkspaceColumns(ctx, database); err != nil {
		database.Close()
		return nil, err
	}
	if err := repository.Seed(database); err != nil {
		database.Close()
		return nil, err
	}
	return database, nil
}

func assertEq(t *testing.T, field, want, got string) {
	t.Helper()
	if want != got {
		t.Fatalf("%s: want %q got %q", field, want, got)
	}
}
