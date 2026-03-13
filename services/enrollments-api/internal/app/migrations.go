package app

import (
	"context"
	"database/sql"

	"proyecto-cursos/internal/platform/migrate"
	embeddedmigrations "proyecto-cursos/services/enrollments-api/migrations"
)

func RunMigrations(ctx context.Context, db *sql.DB) error {
	return migrate.RunFS(ctx, db, embeddedmigrations.Files, "enrollments_schema_migrations")
}
