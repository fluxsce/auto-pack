package db

import (
	"context"
	"database/sql"
	"fmt"

	"auto-pack/backend/internal/repository"
	"auto-pack/backend/pkg/projectcfg"
)

// ensurePackProjectWorkspaceColumns adds workspace/version columns and indexes (idempotent).
func ensurePackProjectWorkspaceColumns(ctx context.Context, db *sql.DB) error {
	exists, err := tableExists(ctx, db, "AP_PACK_PROJECT")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}

	hasWS, err := columnExists(ctx, db, "AP_PACK_PROJECT", "workspace_rec_id")
	if err != nil {
		return err
	}
	if !hasWS {
		for _, q := range []string{
			`ALTER TABLE AP_PACK_PROJECT ADD COLUMN workspace_rec_id TEXT NOT NULL DEFAULT ''`,
			`ALTER TABLE AP_PACK_PROJECT ADD COLUMN project_version TEXT NOT NULL DEFAULT '1.0.0'`,
		} {
			if _, err := db.ExecContext(ctx, q); err != nil {
				return fmt.Errorf("alter AP_PACK_PROJECT: %w", err)
			}
		}
	}

	stmts := []string{
		`DROP INDEX IF EXISTS IX_AP_PACK_PROJ_NAME`,
		`DROP INDEX IF EXISTS IX_AP_PACK_PROJ_CODE`,
		`DROP INDEX IF EXISTS IX_AP_PACK_PROJ_WS_VER`,
		`CREATE UNIQUE INDEX IF NOT EXISTS IX_AP_PACK_PROJ_WS_VER
			ON AP_PACK_PROJECT(workspace_rec_id, project_name, project_version)`,
		`CREATE INDEX IF NOT EXISTS IX_AP_PACK_PROJ_WS ON AP_PACK_PROJECT(workspace_rec_id)`,
		`UPDATE AP_PACK_PROJECT SET workspace_rec_id = ?, project_version = '1.0.0'
			WHERE workspace_rec_id = '' OR workspace_rec_id IS NULL`,
	}
	for i, q := range stmts {
		var execErr error
		if i == len(stmts)-1 {
			_, execErr = db.ExecContext(ctx, q, repository.DefaultWorkspaceRecID)
		} else {
			_, execErr = db.ExecContext(ctx, q)
		}
		if execErr != nil {
			return fmt.Errorf("pack project migrate stmt %d: %w", i+1, execErr)
		}
	}

	hasCode, err := columnExists(ctx, db, "AP_PACK_PROJECT", "project_code")
	if err != nil {
		return err
	}
	if hasCode {
		if _, err := db.ExecContext(ctx, `ALTER TABLE AP_PACK_PROJECT DROP COLUMN project_code`); err != nil {
			return fmt.Errorf("drop project_code column: %w", err)
		}
	}
	return normalizeProjectWorkDirs(ctx, db)
}

func normalizeProjectWorkDirs(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, `SELECT rec_id, project_name, work_dir FROM AP_PACK_PROJECT`)
	if err != nil {
		return fmt.Errorf("list projects for work_dir normalize: %w", err)
	}

	type projectWorkDir struct {
		recID   string
		workDir string
	}
	updates := make([]projectWorkDir, 0)

	for rows.Next() {
		var recID, name, current string
		if err := rows.Scan(&recID, &name, &current); err != nil {
			_ = rows.Close()
			return err
		}
		workDir := projectcfg.SuggestWorkDir(name)
		if workDir == "" || workDir == current {
			continue
		}
		updates = append(updates, projectWorkDir{recID: recID, workDir: workDir})
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(updates) == 0 {
		return nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin work_dir normalize tx: %w", err)
	}

	stmt := `UPDATE AP_PACK_PROJECT SET work_dir = ? WHERE rec_id = ?`
	for _, item := range updates {
		if _, err := tx.ExecContext(ctx, stmt, item.workDir, item.recID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("update work_dir for %s: %w", item.recID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit work_dir normalize: %w", err)
	}
	return nil
}

func tableExists(ctx context.Context, db *sql.DB, name string) (bool, error) {
	var n int
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?`, name,
	).Scan(&n)
	return n > 0, err
}

func columnExists(ctx context.Context, db *sql.DB, table, column string) (bool, error) {
	rows, err := db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dfltValue sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err != nil {
			return false, err
		}
		if name == column {
			return true, nil
		}
	}
	return false, rows.Err()
}
