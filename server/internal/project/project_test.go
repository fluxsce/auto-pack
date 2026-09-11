package project_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/access"
	"auto-pack/server/internal/iam"
	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/project"
	"auto-pack/server/internal/slug"
	"auto-pack/server/internal/store"
	"auto-pack/server/internal/user"
	"auto-pack/server/internal/workspace"
)

func setupPrj(t *testing.T) (*project.Service, *store.Store, context.Context, iam.Actor) {
	t.Helper()
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := migrate.Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	if err := user.SeedDefaultAdmin(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := workspace.SeedDefaultWorkspace(ctx, st); err != nil {
		t.Fatal(err)
	}
	guard := iam.NewGuard(access.NewDirectory(st))
	return project.NewService(st, guard), st, ctx, iam.Actor{UserID: access.DefaultAdminRecID, LoginName: "admin"}
}

func TestSaveRejectsUnsafeProjectName(t *testing.T) {
	svc, _, ctx, actor := setupPrj(t)
	_, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "我的工程",
		ProjectType:    "maven",
	})
	if !errors.Is(err, slug.ErrInvalidSlug) {
		t.Fatalf("want slug, got %v", err)
	}
	p, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "demo-app",
		ProjectType:    "maven",
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.ProjectName != "demo-app" {
		t.Fatalf("name=%s", p.ProjectName)
	}
	ver, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "V9.0.0-P09",
		ProjectType:    "custom",
		WorkRelPath:    "V9.0.0-P09",
		RecipeJSON:     `{"scm_mode":"local","local_source_path":"/home/appinstall/V9.0.0-P09","command":"echo ok"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if ver.ProjectName != "V9.0.0-P09" || ver.WorkRelPath != "V9.0.0-P09" {
		t.Fatalf("ver=%+v", ver)
	}
}

func TestSaveGitRequiresURLAndSetsWorkRel(t *testing.T) {
	svc, _, ctx, actor := setupPrj(t)
	_, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "git-app",
		ProjectType:    "maven",
		RecipeJSON:     `{"scm_mode":"git","command":"mvn -B package"}`,
	})
	if !errors.Is(err, project.ErrRepoURL) {
		t.Fatalf("want repo url, got %v", err)
	}
	p, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "git-app",
		ProjectType:    "maven",
		RepoURL:        "https://git.example.com/org/app.git",
		RecipeJSON:     `{"scm_mode":"git","command":"mvn -B package"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.WorkRelPath != "git-app" {
		t.Fatalf("workrel=%s", p.WorkRelPath)
	}
}

func TestSaveLocalAndCron(t *testing.T) {
	svc, _, ctx, actor := setupPrj(t)
	_, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "local-app",
		ProjectType:    "maven",
		CronExpr:       "bad",
		RecipeJSON:     `{"scm_mode":"local","local_source_path":"src"}`,
	})
	if !errors.Is(err, project.ErrCron) {
		t.Fatalf("want cron, got %v", err)
	}
	p, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "local-app",
		ProjectType:    "maven",
		CronExpr:       "15 3 * * 1",
		RecipeJSON:     `{"scm_mode":"local","local_source_path":"src","command":"mvn -B package"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if p.WorkRelPath != "local-app/src" {
		t.Fatalf("workrel=%s", p.WorkRelPath)
	}
	if p.RepoURL != "" {
		t.Fatalf("repo=%s", p.RepoURL)
	}
}

func TestListIncludesBranchStatusAndTime(t *testing.T) {
	svc, st, ctx, actor := setupPrj(t)
	p, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "status-app",
		ProjectType:    "maven",
		RepoURL:        "https://git.example.com/org/status-app.git",
		RepoBranch:     "release",
		RecipeJSON:     `{"scm_mode":"git","command":"mvn -B package"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	q := st.Rebind(`INSERT INTO AP_JOB (rec_id, workspace_rec_id, project_rec_id, job_type, job_status, payload_json, created_by, finished_at)
		VALUES (?, ?, ?, 'build', 'succeeded', '{}', ?, '2026-09-08 10:00:00')`)
	if _, err := st.SQL().ExecContext(ctx, q, access.NewID(), access.DefaultWorkspaceRecID, p.RecID, actor.UserID); err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(ctx, actor, access.DefaultWorkspaceRecID)
	if err != nil {
		t.Fatal(err)
	}
	var found project.Project
	for _, item := range list {
		if item.RecID == p.RecID {
			found = item
			break
		}
	}
	if found.RepoBranch != "release" {
		t.Fatalf("branch=%s", found.RepoBranch)
	}
	if found.LastJobStatus != "succeeded" || found.LastJobAt == "" {
		t.Fatalf("job=%s at=%s", found.LastJobStatus, found.LastJobAt)
	}
	if found.CreatedAt == "" || found.UpdatedAt == "" {
		t.Fatalf("created=%s updated=%s", found.CreatedAt, found.UpdatedAt)
	}
}

func TestListLastJobIgnoresFtpPush(t *testing.T) {
	svc, st, ctx, actor := setupPrj(t)
	p, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "push-app",
		ProjectType:    "maven",
		RecipeJSON:     `{"command":"mvn -B package"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	ins := st.Rebind(`INSERT INTO AP_JOB (rec_id, workspace_rec_id, project_rec_id, job_type, job_status, payload_json, created_by, finished_at)
		VALUES (?, ?, ?, ?, ?, '{}', ?, ?)`)
	if _, err := st.SQL().ExecContext(ctx, ins, access.NewID(), access.DefaultWorkspaceRecID, p.RecID, "build", "failed", actor.UserID, "2026-09-08 10:00:00"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SQL().ExecContext(ctx, ins, access.NewID(), access.DefaultWorkspaceRecID, p.RecID, "ftp_push", "succeeded", actor.UserID, "2026-09-08 11:00:00"); err != nil {
		t.Fatal(err)
	}
	list, err := svc.List(ctx, actor, access.DefaultWorkspaceRecID)
	if err != nil {
		t.Fatal(err)
	}
	var found project.Project
	for _, item := range list {
		if item.RecID == p.RecID {
			found = item
			break
		}
	}
	if found.LastJobStatus != "failed" {
		t.Fatalf("last job=%s, ftp_push should not replace build history", found.LastJobStatus)
	}
}

func TestCloneCopiesRecipeWithNewName(t *testing.T) {
	svc, _, ctx, actor := setupPrj(t)
	src, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "src-app",
		ProjectType:    "go",
		RepoURL:        "https://git.example.com/org/src-app.git",
		RepoBranch:     "main",
		RecipeJSON:     `{"scm_mode":"git","command":"go build"}`,
		CronExpr:       "0 2 * * *",
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.Clone(ctx, actor, src.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if out.RecID == "" || out.RecID == src.RecID {
		t.Fatalf("id=%s src=%s", out.RecID, src.RecID)
	}
	if out.ProjectName != "src-app-copy" {
		t.Fatalf("name=%s", out.ProjectName)
	}
	if out.WorkRelPath != "src-app-copy" {
		t.Fatalf("workrel=%s", out.WorkRelPath)
	}
	if out.RepoBranch != "main" || out.ProjectType != "go" || out.CronExpr != "0 2 * * *" {
		t.Fatalf("copy=%+v", out)
	}
	again, err := svc.Clone(ctx, actor, src.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if again.ProjectName != "src-app-copy2" {
		t.Fatalf("second=%s", again.ProjectName)
	}
}

func TestUpdateGitBranchesSkipsLocal(t *testing.T) {
	svc, _, ctx, actor := setupPrj(t)
	git, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "git-one",
		ProjectType:    "maven",
		RepoURL:        "https://git.example.com/org/git-one.git",
		RepoBranch:     "main",
		RecipeJSON:     `{"scm_mode":"git","command":"mvn -B package"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	local, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "local-one",
		ProjectType:    "maven",
		RepoBranch:     "main",
		RecipeJSON:     `{"scm_mode":"local","local_source_path":"src","command":"mvn -B package"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.UpdateGitBranches(ctx, actor, access.DefaultWorkspaceRecID, "release")
	if err != nil {
		t.Fatal(err)
	}
	if out.Updated != 1 || out.Skipped != 1 || out.Branch != "release" {
		t.Fatalf("out=%+v", out)
	}
	gotGit, err := svc.GetRaw(ctx, git.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if gotGit.RepoBranch != "release" {
		t.Fatalf("git=%s", gotGit.RepoBranch)
	}
	gotLocal, err := svc.GetRaw(ctx, local.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if gotLocal.RepoBranch != "main" {
		t.Fatalf("local=%s", gotLocal.RepoBranch)
	}
}

func TestSetRepoBranch(t *testing.T) {
	svc, _, ctx, actor := setupPrj(t)
	p, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "set-branch",
		ProjectType:    "go",
		RepoURL:        "https://git.example.com/org/set-branch.git",
		RepoBranch:     "main",
		RecipeJSON:     `{"scm_mode":"git","command":"go build"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.SetRepoBranch(ctx, p.RecID, "hotfix"); err != nil {
		t.Fatal(err)
	}
	got, err := svc.GetRaw(ctx, p.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RepoBranch != "hotfix" {
		t.Fatalf("branch=%s", got.RepoBranch)
	}
}

func TestUpdatePostActionsUpsertsEmail(t *testing.T) {
	svc, _, ctx, actor := setupPrj(t)
	p, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "mail-app",
		ProjectType:    "maven",
		RepoURL:        "https://git.example.com/org/mail-app.git",
		RecipeJSON:     `{"scm_mode":"git","command":"mvn -B package","post_actions":[{"kind":"ftp","env_rec_id":"f1"}]}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	on := true
	out, err := svc.UpdatePostActions(ctx, actor, access.DefaultWorkspaceRecID, project.PostAction{
		Kind:     "email",
		Enabled:  &on,
		EnvRecID: "smtp1",
		To:       "ops@example.com",
		When:     project.WhenAlways,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Updated < 1 || out.Kind != "email" {
		t.Fatalf("out=%+v", out)
	}
	got, err := svc.GetRaw(ctx, p.RecID)
	if err != nil {
		t.Fatal(err)
	}
	recipe := project.ParseRecipe(got.RecipeJSON)
	if len(recipe.PostActions) != 2 {
		t.Fatalf("actions=%+v", recipe.PostActions)
	}
	if recipe.PostActions[0].Kind != "ftp" || recipe.PostActions[1].Kind != "email" || recipe.PostActions[1].EnvRecID != "smtp1" {
		t.Fatalf("actions=%+v", recipe.PostActions)
	}
}

func TestUpdateCommandsRewritesMavenPackage(t *testing.T) {
	svc, _, ctx, actor := setupPrj(t)
	maven, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "lib-core",
		ProjectType:    "maven",
		RepoURL:        "https://git.example.com/org/lib-core.git",
		RecipeJSON:     `{"scm_mode":"git","command":"mvn -B -DskipTests package"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	node, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "web-ui",
		ProjectType:    "vue",
		RepoURL:        "https://git.example.com/org/web-ui.git",
		RecipeJSON:     `{"scm_mode":"git","command":"npm run build"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.UpdateCommands(ctx, actor, access.DefaultWorkspaceRecID, project.CommandPatch{
		Family:  "maven",
		Find:    "package",
		Replace: "install",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Updated != 1 {
		t.Fatalf("updated=%d skipped=%d", out.Updated, out.Skipped)
	}
	got, err := svc.GetRaw(ctx, maven.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if project.ParseRecipe(got.RecipeJSON).Command != "mvn -B -DskipTests install" {
		t.Fatalf("maven cmd=%s", project.ParseRecipe(got.RecipeJSON).Command)
	}
	web, err := svc.GetRaw(ctx, node.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if project.ParseRecipe(web.RecipeJSON).Command != "npm run build" {
		t.Fatalf("node should stay, got %s", project.ParseRecipe(web.RecipeJSON).Command)
	}
}

func TestUpdateCommandsRewritesCleanDeployGoal(t *testing.T) {
	svc, _, ctx, actor := setupPrj(t)
	p, err := svc.Save(ctx, actor, project.Project{
		WorkspaceRecID: access.DefaultWorkspaceRecID,
		ProjectName:    "storage",
		ProjectType:    "maven",
		RepoURL:        "https://git.example.com/org/storage.git",
		RecipeJSON:     `{"scm_mode":"git","command":"mvn clean deploy -DskipTests","output_dir":"target"}`,
	})
	if err != nil {
		t.Fatal(err)
	}
	out, err := svc.UpdateCommands(ctx, actor, access.DefaultWorkspaceRecID, project.CommandPatch{
		Family: "maven",
		Goal:   "install",
	})
	if err != nil {
		t.Fatal(err)
	}
	if out.Updated != 1 {
		t.Fatalf("updated=%d", out.Updated)
	}
	got, err := svc.GetRaw(ctx, p.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if project.ParseRecipe(got.RecipeJSON).Command != "mvn clean install -DskipTests" {
		t.Fatalf("cmd=%s", project.ParseRecipe(got.RecipeJSON).Command)
	}
}
