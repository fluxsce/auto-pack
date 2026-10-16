package mqlog

import (
	"context"
	"path/filepath"
	"testing"

	"auto-pack/server/internal/migrate"
	"auto-pack/server/internal/store"
)

func TestAppendKeepsLatest300(t *testing.T) {
	ctx := context.Background()
	st, err := store.Open(ctx, store.Config{Driver: "sqlite", Path: filepath.Join(t.TempDir(), "t.db")})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := migrate.Apply(ctx, st, "sqlite"); err != nil {
		t.Fatal(err)
	}
	log := New(st)
	for i := 0; i < Keep+5; i++ {
		if err := log.Append(ctx, Entry{
			EnvRecID:    "env-a",
			Direction:   DirectionProduce,
			QueueRole:   "request",
			QueueName:   "req",
			MessageID:   "m",
			Body:        `{"taskId":"T-1"}`,
			Source:      SourceManual,
			ActorUserID: "user-1",
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := log.Append(ctx, Entry{
		EnvRecID:    "env-b",
		Direction:   DirectionConsume,
		QueueRole:   "result",
		QueueName:   "res",
		Body:        `{"taskId":"T-B"}`,
		Source:      SourceIntake,
		ActorUserID: ActorIntake,
	}); err != nil {
		t.Fatal(err)
	}
	rows, err := log.List(ctx, "env-a")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != Keep {
		t.Fatalf("len=%d", len(rows))
	}
	if rows[0].SeqNum != Keep+5 || rows[0].TaskID != "T-1" {
		t.Fatalf("newest seq=%d task=%s", rows[0].SeqNum, rows[0].TaskID)
	}
	if rows[len(rows)-1].SeqNum != 6 {
		t.Fatalf("oldest seq=%d", rows[len(rows)-1].SeqNum)
	}
	other, err := log.List(ctx, "env-b")
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 || other[0].TaskID != "T-B" || other[0].EnvRecID != "env-b" {
		t.Fatalf("other=%+v", other)
	}
}
