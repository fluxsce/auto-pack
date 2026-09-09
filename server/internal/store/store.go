// Package store 只管理平台库连接与事务，不写表访问。
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// Config is the platform database connection settings.
type Config struct {
	Driver string
	Path   string
	DSN    string
}

// Store holds one *sql.DB (pool or current transaction).
type Store struct {
	db     *sql.DB
	tx     *sql.Tx
	driver string
}

// Open opens the platform database. Callers pass config only.
func Open(ctx context.Context, cfg Config) (*Store, error) {
	driver := NormalizeDriver(cfg.Driver)
	if driver == "" {
		return nil, fmt.Errorf("store: driver is required")
	}
	name, dsn, err := sqlOpen(driver, cfg)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open(name, dsn)
	if err != nil {
		return nil, fmt.Errorf("store open: %w", err)
	}
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store ping: %w", err)
	}
	if driver == "sqlite" {
		_, _ = db.ExecContext(ctx, "PRAGMA foreign_keys = OFF")
		_, _ = db.ExecContext(ctx, "PRAGMA busy_timeout = 5000")
	}
	return &Store{db: db, driver: driver}, nil
}

func sqlOpen(driver string, cfg Config) (name, dsn string, err error) {
	switch driver {
	case "sqlite":
		if err := os.MkdirAll(filepath.Dir(cfg.Path), 0o755); err != nil {
			return "", "", err
		}
		return "sqlite", sqliteDSN(cfg.Path), nil
	case "postgres":
		if strings.TrimSpace(cfg.DSN) == "" {
			return "", "", fmt.Errorf("store: postgres requires database.dsn")
		}
		return "pgx", cfg.DSN, nil
	case "mysql":
		if strings.TrimSpace(cfg.DSN) == "" {
			return "", "", fmt.Errorf("store: mysql requires database.dsn")
		}
		return "mysql", cfg.DSN, nil
	default:
		return "", "", fmt.Errorf("store: unsupported driver %q", driver)
	}
}

func sqliteDSN(path string) string {
	const pragmas = "_pragma=busy_timeout(15000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(OFF)"
	p := filepath.ToSlash(filepath.Clean(path))
	if len(p) >= 2 && p[1] == ':' {
		return "file:///" + p + "?" + pragmas
	}
	return "file:" + p + "?" + pragmas
}

// Close closes the pool. Do not call on a transactional Store.
func (s *Store) Close() error {
	if s == nil || s.db == nil || s.tx != nil {
		return nil
	}
	return s.db.Close()
}

// Driver is the normalized engine name.
func (s *Store) Driver() string {
	if s == nil {
		return ""
	}
	return s.driver
}

// SQL returns the current connection or transaction.
func (s *Store) SQL() dbx {
	if s == nil {
		return nil
	}
	if s.tx != nil {
		return s.tx
	}
	return s.db
}

type dbx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// ExecRaw runs a prewritten SQL statement (DDL / seed).
func (s *Store) ExecRaw(ctx context.Context, query string) error {
	if s == nil || s.SQL() == nil {
		return fmt.Errorf("store: not opened")
	}
	_, err := s.SQL().ExecContext(ctx, query)
	return err
}

// Tx runs fn in a transaction.
func (s *Store) Tx(ctx context.Context, fn func(*Store) error) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("store: not opened")
	}
	if s.tx != nil {
		return fn(s)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	child := &Store{db: s.db, tx: tx, driver: s.driver}
	if err := fn(child); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Rebind rewrites ? placeholders for the current driver.
func (s *Store) Rebind(query string) string {
	if s == nil || s.driver != "postgres" {
		return query
	}
	var b strings.Builder
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			n++
			b.WriteByte('$')
			b.WriteString(fmt.Sprintf("%d", n))
			continue
		}
		b.WriteByte(query[i])
	}
	return b.String()
}

// NormalizeDriver maps aliases to sqlite / postgres / mysql.
func NormalizeDriver(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	switch s {
	case "sqlite", "sqlite3":
		return "sqlite"
	case "postgres", "postgresql", "pg":
		return "postgres"
	case "mysql", "mariadb":
		return "mysql"
	default:
		return s
	}
}
