// Package migrations exposes the SQL migrations embedded into the binary.
//
// The files keep goose's naming and annotation format, so the very same
// migrations can still be applied by hand with the CLI when needed:
//
//	goose -dir migrations/sqlite3 sqlite3 data/edh_stats.db up
package migrations

import "embed"

// FS contains the migration files compiled into the binary.
//
//go:embed sqlite3/*.sql
var FS embed.FS
