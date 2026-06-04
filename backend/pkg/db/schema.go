package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// applyScript splits script on ';' and executes each non-empty statement (see splitSQLStatements).
func applyScript(ctx context.Context, db *sql.DB, script []byte) error {
	stmts := splitSQLStatements(string(script))
	for i, q := range stmts {
		if _, err := db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("stmt %d: %w", i+1, err)
		}
	}
	return nil
}

func splitSQLStatements(script string) []string {
	script = strings.TrimSpace(script)
	if script == "" {
		return nil
	}
	var out []string
	for _, ch := range strings.Split(script, ";") {
		s := strings.TrimSpace(stripLeadingLineComments(ch))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func stripLeadingLineComments(s string) string {
	lines := strings.Split(s, "\n")
	i := 0
	for i < len(lines) {
		t := strings.TrimSpace(lines[i])
		if t == "" || strings.HasPrefix(t, "--") {
			i++
			continue
		}
		break
	}
	return strings.Join(lines[i:], "\n")
}
