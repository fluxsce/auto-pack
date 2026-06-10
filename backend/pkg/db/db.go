package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"auto-pack/backend/internal/repository"

	_ "modernc.org/sqlite"
)

// Open opens SQLite and runs migrations plus seed data.
func Open(dbPath string) (*sql.DB, error) {
	dsn := sqliteOpenDSN(dbPath)
	if err := ensureSQLiteDirs(sqliteFilePath(dbPath)); err != nil {
		return nil, err
	}

	database, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sqlite open: %w", err)
	}
	configureSQLitePool(dsn, database)

	if err := database.Ping(); err != nil {
		database.Close()
		return nil, fmt.Errorf("sqlite ping: %w", err)
	}

	if err := migrate(database); err != nil {
		database.Close()
		return nil, err
	}
	if err := repository.Seed(database); err != nil {
		database.Close()
		return nil, err
	}

	return database, nil
}

func migrate(db *sql.DB) error {
	scripts, err := resolveSchemaScripts()
	if err != nil {
		return err
	}
	ctx := context.Background()
	for _, path := range scripts {
		schemaSQL, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read schema script failed (%s): %w", path, err)
		}
		if err := applyScript(ctx, db, schemaSQL); err != nil {
			return fmt.Errorf("execute schema script failed (%s): %w", path, err)
		}
	}
	if err := ensurePackProjectWorkspaceColumns(ctx, db); err != nil {
		return fmt.Errorf("pack project workspace migrate: %w", err)
	}
	if err := ensureReleasePostOptionsColumn(ctx, db); err != nil {
		return fmt.Errorf("workspace release migrate: %w", err)
	}
	return nil
}

func resolveSchemaScripts() ([]string, error) {
	dir, err := resolveSchemaDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read schema dir %s: %w", dir, err)
	}
	var scripts []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		scripts = append(scripts, filepath.Join(dir, e.Name()))
	}
	if len(scripts) == 0 {
		return nil, fmt.Errorf("no schema scripts in %s", dir)
	}
	sort.Strings(scripts)
	return scripts, nil
}

func resolveSchemaDir() (string, error) {
	rel := filepath.Join("scripts", "sqlite")
	candidates := []string{
		rel,
		filepath.Join("..", rel),
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(exe), rel)}, candidates...)
	}
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("schema script dir not found (tried: %v)", candidates)
}

func ensureSQLiteDirs(filePath string) error {
	if filePath == "" {
		return nil
	}
	dir := filepath.Dir(filePath)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func configureSQLitePool(dsn string, database *sql.DB) {
	if strings.Contains(dsn, ":memory:") || strings.Contains(dsn, "mode=memory") {
		database.SetMaxOpenConns(1)
		database.SetMaxIdleConns(1)
		return
	}
	database.SetMaxOpenConns(16)
	database.SetMaxIdleConns(8)
	database.SetConnMaxLifetime(30 * time.Minute)
}
