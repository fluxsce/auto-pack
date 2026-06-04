package db

import (
	"runtime"
	"strings"
	"testing"
)

func TestSQLiteOpenDSN(t *testing.T) {
	t.Parallel()
	cases := []struct {
		in  string
		out string
	}{
		{"mem", ":memory:"},
		{":memory:", ":memory:"},
		{
			"./data/db/autopack.db",
			"file:data/db/autopack.db?" + sqlitePragmaQuery,
		},
	}
	for _, tc := range cases {
		got := sqliteOpenDSN(tc.in)
		if got != tc.out {
			t.Fatalf("sqliteOpenDSN(%q) = %q, want %q", tc.in, got, tc.out)
		}
		if tc.out != ":memory:" && !strings.Contains(got, "busy_timeout") {
			t.Fatalf("expected busy_timeout pragma in %q", got)
		}
	}
}

func TestSQLiteOpenDSN_windowsDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only")
	}
	got := sqliteOpenDSN(`E:\data\autopack.db`)
	want := "file:///E:/data/autopack.db?" + sqlitePragmaQuery
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestSQLiteFilePath(t *testing.T) {
	t.Parallel()
	if got := sqliteFilePath(`E:\data\autopack.db`); got != `E:\data\autopack.db` {
		t.Fatalf("got %q", got)
	}
	if got := sqliteFilePath("mem"); got != "" {
		t.Fatalf("mem should be empty path, got %q", got)
	}
}
