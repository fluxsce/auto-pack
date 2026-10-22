package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"auto-pack/server/internal/store"
)

// retargetIntakeKey 给已有 AP_INTAKE_TASK 补上 rec_id，并把主键改到这一列。
// 新库建表时主键已经是 rec_id，这里直接返回。
// pack_task_id 可以重复：同一报文再次投递会再打包。空值用 task_id 补上，便于按打包号查找。
func retargetIntakeKey(ctx context.Context, st *store.Store) error {
	if st == nil {
		return nil
	}
	switch st.Driver() {
	case "sqlite":
		return retargetIntakeSQLite(ctx, st)
	case "mysql":
		return retargetIntakeMySQL(ctx, st)
	case "postgres":
		return retargetIntakePostgres(ctx, st)
	default:
		return nil
	}
}

func retargetIntakeSQLite(ctx context.Context, st *store.Store) error {
	var name string
	err := st.SQL().QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='AP_INTAKE_TASK'`).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	pk, err := intakePK(ctx, st, `SELECT name FROM pragma_table_info('AP_INTAKE_TASK') WHERE pk > 0 ORDER BY pk`)
	if err != nil || pk == "rec_id" {
		return err
	}
	if err := backfillIntakePack(ctx, st); err != nil {
		return err
	}
	if err := ensureIntakeRecID(ctx, st, pk); err != nil {
		return err
	}
	return st.Tx(ctx, func(tx *store.Store) error {
		stmts := []string{
			`CREATE TABLE AP_INTAKE_TASK_NEW (
				rec_id TEXT NOT NULL,
				task_id TEXT NOT NULL,
				task_no TEXT,
				pack_task_id TEXT NOT NULL,
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
				CONSTRAINT PK_AP_INTAKE PRIMARY KEY (rec_id)
			)`,
			`INSERT INTO AP_INTAKE_TASK_NEW (
				rec_id, task_id, task_no, pack_task_id, subsystem_type, project_rec_id, job_rec_id,
				task_status, result_status, result_message, oss_project_root, notice_json,
				finished_at, created_at, updated_at)
				SELECT rec_id, task_id, task_no, pack_task_id, subsystem_type, project_rec_id, job_rec_id,
				task_status, result_status, result_message, oss_project_root, notice_json,
				finished_at, created_at, updated_at FROM AP_INTAKE_TASK`,
			`DROP TABLE AP_INTAKE_TASK`,
			`ALTER TABLE AP_INTAKE_TASK_NEW RENAME TO AP_INTAKE_TASK`,
			`CREATE INDEX IF NOT EXISTS IDX_AP_INTAKE_TASK ON AP_INTAKE_TASK (task_id)`,
			`CREATE INDEX IF NOT EXISTS IDX_AP_INTAKE_PACK ON AP_INTAKE_TASK (pack_task_id)`,
		}
		for _, q := range stmts {
			if _, err := tx.SQL().ExecContext(ctx, q); err != nil {
				return err
			}
		}
		return nil
	})
}

func retargetIntakeMySQL(ctx context.Context, st *store.Store) error {
	pk, err := intakePK(ctx, st, `SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = DATABASE() AND LOWER(TABLE_NAME) = 'ap_intake_task' AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION`)
	if err != nil || pk == "" || pk == "rec_id" {
		return err
	}
	if err := backfillIntakePack(ctx, st); err != nil {
		return err
	}
	if err := ensureIntakeRecID(ctx, st, pk); err != nil {
		return err
	}
	if _, err := st.SQL().ExecContext(ctx, `ALTER TABLE AP_INTAKE_TASK MODIFY rec_id VARCHAR(36) NOT NULL`); err != nil {
		return err
	}
	if _, err := st.SQL().ExecContext(ctx, `ALTER TABLE AP_INTAKE_TASK MODIFY pack_task_id VARCHAR(128) NOT NULL`); err != nil {
		return err
	}
	if _, err := st.SQL().ExecContext(ctx, `ALTER TABLE AP_INTAKE_TASK DROP PRIMARY KEY, ADD PRIMARY KEY (rec_id)`); err != nil {
		return err
	}
	for _, q := range []string{
		`CREATE INDEX IDX_AP_INTAKE_TASK ON AP_INTAKE_TASK (task_id)`,
		`CREATE INDEX IDX_AP_INTAKE_PACK ON AP_INTAKE_TASK (pack_task_id)`,
	} {
		if _, err := st.SQL().ExecContext(ctx, q); err != nil && !isDupOrNoop(err) {
			return err
		}
	}
	return nil
}

func retargetIntakePostgres(ctx context.Context, st *store.Store) error {
	pk, err := intakePK(ctx, st, `SELECT a.attname
		FROM pg_index i
		JOIN pg_attribute a ON a.attrelid = i.indrelid AND a.attnum = ANY(i.indkey)
		WHERE i.indrelid = '"AP_INTAKE_TASK"'::regclass AND i.indisprimary
		ORDER BY a.attnum`)
	if err != nil || pk == "" || pk == "rec_id" {
		return err
	}
	if err := backfillIntakePack(ctx, st); err != nil {
		return err
	}
	if err := ensureIntakeRecID(ctx, st, pk); err != nil {
		return err
	}
	if _, err := st.SQL().ExecContext(ctx, `ALTER TABLE "AP_INTAKE_TASK" ALTER COLUMN "rec_id" SET NOT NULL`); err != nil {
		return err
	}
	if _, err := st.SQL().ExecContext(ctx, `ALTER TABLE "AP_INTAKE_TASK" ALTER COLUMN "pack_task_id" SET NOT NULL`); err != nil {
		return err
	}
	if _, err := st.SQL().ExecContext(ctx, `ALTER TABLE "AP_INTAKE_TASK" DROP CONSTRAINT IF EXISTS "PK_AP_INTAKE"`); err != nil {
		return err
	}
	if _, err := st.SQL().ExecContext(ctx, `ALTER TABLE "AP_INTAKE_TASK" ADD CONSTRAINT "PK_AP_INTAKE" PRIMARY KEY ("rec_id")`); err != nil {
		return err
	}
	_, err = st.SQL().ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "IDX_AP_INTAKE_TASK" ON "AP_INTAKE_TASK" ("task_id")`)
	if err != nil {
		return err
	}
	_, err = st.SQL().ExecContext(ctx, `CREATE INDEX IF NOT EXISTS "IDX_AP_INTAKE_PACK" ON "AP_INTAKE_TASK" ("pack_task_id")`)
	return err
}

func backfillIntakePack(ctx context.Context, st *store.Store) error {
	q := `UPDATE AP_INTAKE_TASK SET pack_task_id = task_id WHERE pack_task_id IS NULL OR trim(pack_task_id) = ''`
	if st.Driver() == "postgres" {
		q = `UPDATE "AP_INTAKE_TASK" SET "pack_task_id" = "task_id" WHERE "pack_task_id" IS NULL OR btrim("pack_task_id") = ''`
	}
	_, err := st.SQL().ExecContext(ctx, q)
	return err
}

func ensureIntakeRecID(ctx context.Context, st *store.Store, pk string) error {
	has, err := intakeHasColumn(ctx, st, "rec_id")
	if err != nil {
		return err
	}
	if !has {
		q := `ALTER TABLE AP_INTAKE_TASK ADD COLUMN rec_id VARCHAR(36)`
		switch st.Driver() {
		case "sqlite":
			q = `ALTER TABLE AP_INTAKE_TASK ADD COLUMN rec_id TEXT`
		case "postgres":
			q = `ALTER TABLE "AP_INTAKE_TASK" ADD COLUMN "rec_id" VARCHAR(36)`
		}
		if _, err := st.SQL().ExecContext(ctx, q); err != nil {
			return err
		}
	}
	if pk != "task_id" && pk != "pack_task_id" {
		return fmt.Errorf("migrate: AP_INTAKE_TASK primary key is %s", pk)
	}
	return fillIntakeRecIDs(ctx, st, pk)
}

func fillIntakeRecIDs(ctx context.Context, st *store.Store, keyCol string) error {
	table, col := "AP_INTAKE_TASK", keyCol
	where := `rec_id IS NULL OR trim(rec_id) = ''`
	if st.Driver() == "postgres" {
		table = `"AP_INTAKE_TASK"`
		col = `"` + keyCol + `"`
		where = `"rec_id" IS NULL OR btrim("rec_id") = ''`
	}
	rows, err := st.SQL().QueryContext(ctx, `SELECT `+col+` FROM `+table+` WHERE `+where)
	if err != nil {
		return err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return err
		}
		keys = append(keys, key)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	upd := st.Rebind(`UPDATE ` + table + ` SET rec_id=? WHERE ` + col + `=?`)
	if st.Driver() == "postgres" {
		upd = st.Rebind(`UPDATE "AP_INTAKE_TASK" SET "rec_id"=? WHERE ` + col + `=?`)
	}
	for _, key := range keys {
		if _, err := st.SQL().ExecContext(ctx, upd, uuid.NewString(), key); err != nil {
			return err
		}
	}
	return nil
}

func intakeHasColumn(ctx context.Context, st *store.Store, column string) (bool, error) {
	var q string
	switch st.Driver() {
	case "sqlite":
		q = `SELECT 1 FROM pragma_table_info('AP_INTAKE_TASK') WHERE lower(name)=?`
	case "mysql":
		q = `SELECT 1 FROM information_schema.COLUMNS
			WHERE TABLE_SCHEMA = DATABASE() AND LOWER(TABLE_NAME) = 'ap_intake_task' AND LOWER(COLUMN_NAME) = ?`
	case "postgres":
		q = `SELECT 1 FROM information_schema.columns
			WHERE table_schema = current_schema() AND table_name = 'AP_INTAKE_TASK' AND column_name = ?`
	default:
		return false, nil
	}
	var one int
	err := st.SQL().QueryRowContext(ctx, st.Rebind(q), strings.ToLower(column)).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func intakePK(ctx context.Context, st *store.Store, query string) (string, error) {
	rows, err := st.SQL().QueryContext(ctx, query)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return "", err
		}
		cols = append(cols, strings.ToLower(strings.TrimSpace(name)))
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	if len(cols) != 1 {
		if len(cols) == 0 {
			return "", nil
		}
		return "", fmt.Errorf("migrate: AP_INTAKE_TASK primary key is %s", strings.Join(cols, ","))
	}
	return cols[0], nil
}
