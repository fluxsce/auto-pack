package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestApplyScript_initSchema(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	script := `
BEGIN;
CREATE TABLE IF NOT EXISTS AP_USER_AUTH (
  rec_id TEXT NOT NULL PRIMARY KEY,
  login_name TEXT NOT NULL DEFAULT '',
  is_enabled INTEGER NOT NULL DEFAULT 1 CHECK (is_enabled IN (0,1)),
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE UNIQUE INDEX IF NOT EXISTS IX_AP_USER_LOGIN ON AP_USER_AUTH(login_name);
COMMIT;
`
	if err := applyScript(context.Background(), database, []byte(script)); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := database.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name = 'AP_USER_AUTH'`).Scan(&n); err != nil || n != 1 {
		t.Fatalf("table missing: err=%v n=%d", err, n)
	}
}

func TestApplyScript_repoInitSchemaFile(t *testing.T) {
	path := filepath.Join("..", "..", "..", "scripts", "sqlite", "001_init_schema.sql")
	script, err := os.ReadFile(path)
	if err != nil {
		t.Skip(err)
	}

	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	if err := applyScript(context.Background(), database, script); err != nil {
		t.Fatal(err)
	}
}
