package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenSQLiteAndRebind(t *testing.T) {
	ctx := context.Background()
	st, err := Open(ctx, Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if st.Driver() != "sqlite" {
		t.Fatalf("driver=%s", st.Driver())
	}
	if st.Rebind("SELECT ?") != "SELECT ?" {
		t.Fatal(st.Rebind("SELECT ?"))
	}
	pg := &Store{driver: "postgres"}
	if got := pg.Rebind("SELECT ? FROM t WHERE a=?"); got != "SELECT $1 FROM t WHERE a=$2" {
		t.Fatalf("rebind=%s", got)
	}
}

func TestNormalizeDriver(t *testing.T) {
	if NormalizeDriver("postgresql") != "postgres" {
		t.Fatal(NormalizeDriver("postgresql"))
	}
	if NormalizeDriver("sqlite3") != "sqlite" {
		t.Fatal(NormalizeDriver("sqlite3"))
	}
}
