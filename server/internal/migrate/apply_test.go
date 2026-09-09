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

func TestNormalizeDriversHaveInit(t *testing.T) {
	for _, d := range []string{"sqlite", "postgres", "mysql"} {
		if _, err := FindInitSQL(d); err != nil {
			t.Fatalf("%s: %v", d, err)
		}
	}
}
