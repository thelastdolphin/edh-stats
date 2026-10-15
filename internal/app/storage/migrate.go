package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/pressly/goose/v3"

	"github.com/thelastdolphin/edh-stats/migrations"
)

// dialect is the goose dialect applied at startup. The service only speaks
// SQLite; the postgres directory is kept for a future backend.
const dialect = "sqlite3"

// migrate applies every pending migration embedded in the binary.
//
// The goose library is used instead of shelling out to the goose CLI so that
// deployment stays a single self-contained binary, and so that the same
// goose_db_version bookkeeping and file format the project already committed is
// reused: databases migrated by hand are recognised as up to date.
func migrate(ctx context.Context, db *sql.DB) error {
	goose.SetBaseFS(migrations.FS)

	if err := goose.SetDialect(dialect); err != nil {
		return fmt.Errorf("set goose dialect: %w", err)
	}
	// goose's default logger is left in place on purpose: on startup it reports
	// which migrations were applied, which is what an operator wants to see in
	// the container log.
	if err := goose.UpContext(ctx, db, dialect); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}
