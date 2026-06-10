package db

import (
	"context"
	"database/sql"
	"fmt"
)

func ensureReleasePostOptionsColumn(ctx context.Context, db *sql.DB) error {
	exists, err := tableExists(ctx, db, "AP_PACK_WORKSPACE_RELEASE")
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	has, err := columnExists(ctx, db, "AP_PACK_WORKSPACE_RELEASE", "post_options_json")
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	if _, err := db.ExecContext(ctx,
		`ALTER TABLE AP_PACK_WORKSPACE_RELEASE ADD COLUMN post_options_json TEXT NOT NULL DEFAULT ''`,
	); err != nil {
		return fmt.Errorf("alter AP_PACK_WORKSPACE_RELEASE post_options_json: %w", err)
	}
	return nil
}
