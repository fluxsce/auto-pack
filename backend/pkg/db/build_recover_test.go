package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"auto-pack/backend/internal/repository"
)

func TestRecoverInterruptedBuilds(t *testing.T) {
	database, err := openTestDBWithMigrations(t)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	builds := repository.NewBuild(database)
	projects := repository.NewProject(database)

	projID := "11111111-1111-4111-8111-111111111111"
	if err := projects.Insert(&repository.PackProject{
		RecID:          projID,
		WorkspaceRecID: repository.DefaultWorkspaceRecID,
		ProjectName:    "demo",
		ProjectVersion: "1.0.0",
		ProjectType:    "custom",
		ConfigJSON:     `{}`,
		IsEnabled:      1,
	}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	logPath := filepath.Join(dir, "stale.log")
	if err := os.WriteFile(logPath, []byte("partial\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	stale := &repository.PackBuild{
		RecID:        "22222222-2222-4222-8222-222222222222",
		ProjectRecID: projID,
		BuildNum:     1,
		Status:       repository.BuildStatusRunning,
		Message:      "running",
		LogPath:      logPath,
		StartedAt:    &now,
	}
	if err := builds.Insert(stale); err != nil {
		t.Fatal(err)
	}

	n, err := builds.RecoverInterrupted("")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("rows affected = %d, want 1", n)
	}

	got, err := builds.FindByRecID(stale.RecID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != repository.BuildStatusFailed {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.FinishedAt == nil {
		t.Fatal("expected finished_at to be set")
	}

	running, err := builds.FindRunningByProject(projID)
	if err != nil {
		t.Fatal(err)
	}
	if running != nil {
		t.Fatal("expected no running build after recovery")
	}
}
