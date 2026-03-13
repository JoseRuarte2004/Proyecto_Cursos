package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

func RunFS(ctx context.Context, db *sql.DB, migrations fs.FS, tableName string) error {
	if _, err := db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, tableName)); err != nil {
		return err
	}

	entries, err := fs.ReadDir(migrations, ".")
	if err != nil {
		return err
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		var applied bool
		query := fmt.Sprintf(`SELECT EXISTS(SELECT 1 FROM %s WHERE version = $1)`, tableName)
		if err := db.QueryRowContext(ctx, query, entry.Name()).Scan(&applied); err != nil {
			return err
		}

		if applied {
			continue
		}

		script, err := fs.ReadFile(migrations, entry.Name())
		if err != nil {
			return err
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, string(script)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}

		insertQuery := fmt.Sprintf(`INSERT INTO %s (version) VALUES ($1)`, tableName)
		if _, err := tx.ExecContext(ctx, insertQuery, entry.Name()); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}
	}

	return nil
}
