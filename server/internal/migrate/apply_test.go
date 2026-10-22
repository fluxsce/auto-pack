package migrate

import (
	"context"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/store"
)

func TestApplySQLite(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(dir, "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	if _, err := Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := st.SQL().QueryRowContext(ctx, `SELECT COUNT(1) FROM AP_ROLE_ACTION`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n < 6 {
		t.Fatalf("seed actions=%d", n)
	}
}

func TestRetargetIntakePrimaryKey(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := st.SQL().ExecContext(ctx, `CREATE TABLE AP_INTAKE_TASK (
		task_id TEXT NOT NULL,
		task_no TEXT,
		pack_task_id TEXT,
		subsystem_type TEXT,
		project_rec_id TEXT,
		job_rec_id TEXT,
		task_status TEXT NOT NULL,
		result_status TEXT,
		result_message TEXT,
		oss_project_root TEXT,
		notice_json TEXT,
		finished_at TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		CONSTRAINT PK_AP_INTAKE PRIMARY KEY (task_id)
	)`); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SQL().ExecContext(ctx, `INSERT INTO AP_INTAKE_TASK (
		task_id, task_no, pack_task_id, subsystem_type, project_rec_id, job_rec_id,
		task_status, result_status, result_message, oss_project_root, notice_json, finished_at, created_at, updated_at)
		VALUES ('task-a','','','','','','running','','','','','','2026-01-01','2026-01-01'),
		       ('task-b','','PACK-B','','','','running','','','','','','2026-01-01','2026-01-01'),
		       ('task-old','','PACK-DUP','','','','running','','','','','','2026-01-01','2026-01-01'),
		       ('task-new','','PACK-DUP','','','','succeeded','','','','','','2026-01-03','2026-01-03')`); err != nil {
		t.Fatal(err)
	}
	if err := retargetIntakeKey(ctx, st); err != nil {
		t.Fatal(err)
	}
	if err := retargetIntakeKey(ctx, st); err != nil {
		t.Fatal(err)
	}
	var pack string
	if err := st.SQL().QueryRowContext(ctx, `SELECT pack_task_id FROM AP_INTAKE_TASK WHERE task_id='task-a'`).Scan(&pack); err != nil {
		t.Fatal(err)
	}
	if pack != "task-a" {
		t.Fatalf("backfill=%s", pack)
	}
	var oldPack, newPack, oldID, newID, pk string
	if err := st.SQL().QueryRowContext(ctx, `SELECT pack_task_id, rec_id FROM AP_INTAKE_TASK WHERE task_id='task-new'`).Scan(&newPack, &newID); err != nil {
		t.Fatal(err)
	}
	if err := st.SQL().QueryRowContext(ctx, `SELECT pack_task_id, rec_id FROM AP_INTAKE_TASK WHERE task_id='task-old'`).Scan(&oldPack, &oldID); err != nil {
		t.Fatal(err)
	}
	if newPack != "PACK-DUP" || oldPack != "PACK-DUP" {
		t.Fatalf("dup pack old=%s new=%s", oldPack, newPack)
	}
	if oldID == "" || newID == "" || oldID == newID {
		t.Fatalf("rec_id old=%s new=%s", oldID, newID)
	}
	if err := st.SQL().QueryRowContext(ctx, `SELECT name FROM pragma_table_info('AP_INTAKE_TASK') WHERE pk > 0`).Scan(&pk); err != nil {
		t.Fatal(err)
	}
	if pk != "rec_id" {
		t.Fatalf("pk=%s", pk)
	}
	if _, err := st.SQL().ExecContext(ctx, `INSERT INTO AP_INTAKE_TASK (
		rec_id, task_id, pack_task_id, task_status, created_at, updated_at)
		VALUES ('row-2','task-a','PACK-2','running','2026-01-02','2026-01-02')`); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeDriversHaveInit(t *testing.T) {
	for _, d := range []string{"sqlite", "postgres", "mysql"} {
		if _, err := FindInitSQL(d); err != nil {
			t.Fatalf("%s: %v", d, err)
		}
	}
}
