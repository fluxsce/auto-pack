package project_test

import (
	"archive/zip"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/project"
)

func TestParseRecipeBindings(t *testing.T) {
	r := project.ParseRecipe(`{
		"command":"mvn -B package",
		"scm_mode":"git",
		"auto_sync":false,
		"output_dir":"target",
		"artifact_pattern":"*.war",
		"git_env_rec_id":"g1",
		"jdk_env_rec_id":"j1",
		"maven_env_rec_id":"m1",
		"ftp_env_rec_id":"f1",
		"remote_path":"/opt/app"
	}`)
	if r.Command == "" || r.GitEnvRecID != "g1" || r.JDKEnvRecID != "j1" || r.FTPEnvRecID != "f1" {
		t.Fatalf("%+v", r)
	}
	if r.ShouldSync() {
		t.Fatal("auto_sync=false")
	}
	if r.IsLocal() {
		t.Fatal("git")
	}
	dir, pat := project.ArtifactSpec(r, "")
	if dir != "target" || pat != "*.war" {
		t.Fatalf("spec %s %s", dir, pat)
	}
}

func TestRecipeLocalSkipsSync(t *testing.T) {
	r := project.ParseRecipe(`{"scm_mode":"local","local_source_path":"src"}`)
	if !r.IsLocal() || r.ShouldSync() {
		t.Fatalf("%+v", r)
	}
	if got := project.WorkRel("demo-app", r); got != "demo-app/src" {
		t.Fatalf("workrel=%s", got)
	}
	abs := project.ParseRecipe(`{"scm_mode":"local","local_source_path":"/home/appinstall/V10.0.0-P05"}`)
	if project.AbsLocalSource(abs) == "" || project.WorkRel("V10.0.0-P05", abs) != "V10.0.0-P05" {
		t.Fatalf("abs local workrel=%s source=%s", project.WorkRel("V10.0.0-P05", abs), project.AbsLocalSource(abs))
	}
	legacy := project.ParseRecipe(`{"command":"echo"}`)
	if legacy.IsLocal() || !legacy.ShouldSync() {
		t.Fatal("legacy defaults to git pull when URL exists")
	}
	if got := project.WorkRel("demo-app", legacy); got != "demo-app" {
		t.Fatalf("workrel=%s", got)
	}
}

func TestCheckRepoURLAndCron(t *testing.T) {
	if err := project.CheckRepoURL(""); !errors.Is(err, project.ErrRepoURL) {
		t.Fatalf("empty: %v", err)
	}
	if err := project.CheckRepoURL("not-a-url"); !errors.Is(err, project.ErrRepoURL) {
		t.Fatalf("bad: %v", err)
	}
	for _, url := range []string{
		"https://git.example.com/org/app.git",
		"git@git.example.com:org/app.git",
		"ssh://git@host/app.git",
	} {
		if err := project.CheckRepoURL(url); err != nil {
			t.Fatalf("%s: %v", url, err)
		}
	}
	if err := project.CheckCron(""); err != nil {
		t.Fatal(err)
	}
	if err := project.CheckCron("0 2 * * *"); err != nil {
		t.Fatal(err)
	}
	if err := project.CheckCron("@daily"); err != nil {
		t.Fatal(err)
	}
	if err := project.CheckCron("not cron"); !errors.Is(err, project.ErrCron) {
		t.Fatalf("bad cron: %v", err)
	}
	if err := project.CheckRepoBranch(""); !errors.Is(err, project.ErrRepoBranch) {
		t.Fatalf("empty branch: %v", err)
	}
	if err := project.CheckRepoBranch("../hack"); !errors.Is(err, project.ErrRepoBranch) {
		t.Fatalf("dotdot: %v", err)
	}
	for _, branch := range []string{"main", "develop", "release/V9.0.0-P01"} {
		if err := project.CheckRepoBranch(branch); err != nil {
			t.Fatalf("%s: %v", branch, err)
		}
	}
}

func TestPreviewCron(t *testing.T) {
	empty := project.PreviewCron("", 5)
	if empty.OK || len(empty.Times) != 0 {
		t.Fatalf("%+v", empty)
	}
	bad := project.PreviewCron("not cron", 5)
	if bad.OK || bad.Error == "" {
		t.Fatalf("%+v", bad)
	}
	got := project.PreviewCron("0 2 * * *", 5)
	if !got.OK || len(got.Times) != 5 || got.TZ == "" {
		t.Fatalf("%+v", got)
	}
	daily := project.PreviewCron("@daily", 3)
	if !daily.OK || len(daily.Times) != 3 {
		t.Fatalf("%+v", daily)
	}
}

func TestCollectArtifacts(t *testing.T) {
	work := t.TempDir()
	target := filepath.Join(work, "target")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "app.jar"), []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "readme.txt"), []byte("txt"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, err := project.CollectArtifacts(work, project.Recipe{OutputDir: "target", ArtifactPattern: "*.jar"}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || filepath.Base(files[0]) != "app.jar" {
		t.Fatalf("%v", files)
	}
	if _, err := project.CollectArtifacts(work, project.Recipe{OutputDir: "target", ArtifactPattern: "*.war"}, ""); !errors.Is(err, project.ErrNoArtifacts) {
		t.Fatalf("want no artifacts, got %v", err)
	}
}

func TestCollectPushPaths(t *testing.T) {
	work := t.TempDir()
	target := filepath.Join(work, "target")
	if err := os.MkdirAll(filepath.Join(target, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "app.jar"), []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "nested", "lib.jar"), []byte("lib"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "pom.xml"), []byte("<p/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, ".git", "HEAD"), []byte("ref"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, cleanup, err := project.CollectPushPaths(work, []string{"pom.xml", "target"})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	names := map[string]bool{}
	var zipPath string
	for _, f := range files {
		names[filepath.Base(f)] = true
		if filepath.Base(f) == "target.zip" {
			zipPath = f
		}
	}
	if !names["pom.xml"] || !names["target.zip"] {
		t.Fatalf("got %v", files)
	}
	if names["app.jar"] || names["HEAD"] {
		t.Fatal("dir should be zipped, .git skipped")
	}
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	inZip := map[string]bool{}
	for _, f := range zr.File {
		inZip[f.Name] = true
	}
	if !inZip["target/app.jar"] || !inZip["target/nested/lib.jar"] {
		t.Fatalf("zip entries %v", inZip)
	}
	if inZip["target/.git/HEAD"] {
		t.Fatal("zip should skip .git")
	}
	if _, _, err := project.CollectPushPaths(work, []string{"../secret"}); err == nil {
		t.Fatal("expected jail")
	}
	if _, _, err := project.CollectPushPaths(work, []string{""}); !errors.Is(err, project.ErrNoArtifacts) {
		t.Fatalf("root: %v", err)
	}

	wars := filepath.Join(work, "target", "app.war")
	if err := os.WriteFile(wars, []byte("war"), 0o644); err != nil {
		t.Fatal(err)
	}
	globFiles, globCleanup, err := project.CollectPushPaths(work, []string{"target/*.war", "target/*.jar"})
	if err != nil {
		t.Fatal(err)
	}
	defer globCleanup()
	globNames := map[string]bool{}
	for _, f := range globFiles {
		globNames[filepath.Base(f)] = true
	}
	if !globNames["app.war"] || !globNames["app.jar"] || globNames["lib.jar"] {
		t.Fatalf("glob got %v", globFiles)
	}
	deep, deepCleanup, err := project.CollectPushPaths(work, []string{"target/**/*.jar"})
	if err != nil {
		t.Fatal(err)
	}
	defer deepCleanup()
	deepNames := map[string]bool{}
	for _, f := range deep {
		deepNames[filepath.Base(f)] = true
	}
	if !deepNames["app.jar"] || !deepNames["lib.jar"] {
		t.Fatalf("** glob got %v", deep)
	}
	if _, _, err := project.CollectPushPaths(work, []string{"*"}); !errors.Is(err, project.ErrNoArtifacts) {
		t.Fatalf("root glob: %v", err)
	}
	if _, _, err := project.CollectPushPaths(work, []string{"missing/*.war"}); !errors.Is(err, project.ErrNoArtifacts) {
		t.Fatalf("empty glob: %v", err)
	}
}

func TestRecipeActionsAndDepends(t *testing.T) {
	legacy := project.ParseRecipe(`{"ftp_env_rec_id":"f1","remote_path":"/opt"}`)
	acts := legacy.Actions()
	if len(acts) != 1 || acts[0].EnvRecID != "f1" || acts[0].RemotePath != "/opt" {
		t.Fatalf("%+v", acts)
	}
	r := project.ParseRecipe(`{"post_actions":[{"kind":"shell","command":"echo ok","enabled":false},{"kind":"ftp","env_rec_id":"f2"}]}`)
	if len(r.Actions()) != 2 || project.ActionEnabled(r.Actions()[0]) {
		t.Fatal(r.Actions())
	}
	if !project.HasSelfDepend([]string{"a", "me"}, "me") || project.HasSelfDepend([]string{"a"}, "me") {
		t.Fatal("self depend")
	}
	if !project.InDependChain([]string{"a", "b"}, "b") || project.InDependChain([]string{"a"}, "c") {
		t.Fatal("chain")
	}
}

func TestActionMatches(t *testing.T) {
	off := false
	disabled := project.PostAction{Kind: "email", Enabled: &off, When: project.WhenAlways}
	if project.ActionMatches(disabled, true) || project.ActionMatches(disabled, false) {
		t.Fatal("disabled")
	}
	if project.ActionWhen(project.PostAction{}) != project.WhenSuccess {
		t.Fatal("default when")
	}
	if !project.ActionMatches(project.PostAction{Kind: "email"}, true) {
		t.Fatal("success default")
	}
	if project.ActionMatches(project.PostAction{Kind: "ftp", When: project.WhenFailure}, true) {
		t.Fatal("failure on ok")
	}
	if !project.ActionMatches(project.PostAction{Kind: "email", When: project.WhenFailure}, false) {
		t.Fatal("failure on fail")
	}
	if !project.ActionMatches(project.PostAction{Kind: "email", When: project.WhenAlways}, false) {
		t.Fatal("always")
	}
	if !project.IsEmailAction("SMTP") || project.IsEmailAction("ftp") {
		t.Fatal("email kind")
	}
	on := true
	if !project.ShouldAttachMailLog(project.PostAction{}, false) {
		t.Fatal("failure always attaches log")
	}
	if project.ShouldAttachMailLog(project.PostAction{}, true) {
		t.Fatal("success without attach_log")
	}
	if !project.ShouldAttachMailLog(project.PostAction{AttachLog: &on}, true) {
		t.Fatal("success with attach_log")
	}
}

func TestUpsertPostActionReplacesSameKind(t *testing.T) {
	if err := project.CheckPostAction(project.PostAction{Kind: "email"}); !errors.Is(err, project.ErrPostEnv) {
		t.Fatalf("email: %v", err)
	}
	if err := project.CheckPostAction(project.PostAction{Kind: "shell"}); !errors.Is(err, project.ErrPostShell) {
		t.Fatalf("shell: %v", err)
	}
	r := project.ParseRecipe(`{"post_actions":[{"id":"old","kind":"email","env_rec_id":"smtp1"},{"kind":"ftp","env_rec_id":"f1"}]}`)
	next := project.UpsertPostAction(r, project.PostAction{Kind: "email", EnvRecID: "smtp2", To: "a@b.c"})
	if len(next.PostActions) != 2 {
		t.Fatalf("%+v", next.PostActions)
	}
	if next.PostActions[0].EnvRecID != "smtp2" || next.PostActions[0].ID != "old" || next.PostActions[1].Kind != "ftp" {
		t.Fatalf("%+v", next.PostActions)
	}
}
