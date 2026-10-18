package storage

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"

	// sqlite3 driver; also registers the "sqlite3" driver name that goose uses.
	_ "github.com/mattn/go-sqlite3"
)

// maxOpenConns keeps the pool at a single connection.
//
// SQLite allows exactly one writer at a time and every connection carries its
// own PRAGMAs. A single connection removes both problems at once: no
// SQLITE_BUSY between concurrent writers, and no pooled connection silently
// running without foreign keys. Raise it (together with the WAL mode enabled
// below) once read concurrency actually matters.
const maxOpenConns = 1

// busyTimeoutMs is how long a statement waits for a lock before failing.
const busyTimeoutMs = 5000

// Store owns the database handle and the repositories built on top of it.
type Store struct {
	db             *sql.DB
	deckRepository *DeckRepository
	gameRepository *GameRepository
}

// NewStore opens (creating it if necessary) the SQLite database at dbPath,
// applies pending migrations and returns a ready to use Store.
func NewStore(ctx context.Context, dbPath string) (*Store, error) {
	db, err := sql.Open("sqlite3", dataSourceName(dbPath))
	if err != nil {
		return nil, fmt.Errorf("open sqlite database %q: %w", dbPath, err)
	}
	db.SetMaxOpenConns(maxOpenConns)

	// sql.Open is lazy, so ping here to surface an invalid path or a missing
	// directory at startup instead of on the first request.
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("connect to sqlite database %q: %w", dbPath, err)
	}

	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	store := &Store{db: db}
	store.deckRepository = &DeckRepository{db: db}
	store.gameRepository = &GameRepository{db: db}
	return store, nil
}

// dataSourceName builds a go-sqlite3 DSN.
//
// The PRAGMAs belong in the DSN rather than in a one-off Exec: SQLite applies
// them per connection, so a pooled connection that never ran the statement
// would silently work without foreign keys (previously only 1 of 5 concurrent
// connections had foreign_keys enabled).
//
//	_foreign_keys  enforce the REFERENCES clauses of the schema
//	_busy_timeout  wait for a lock instead of failing immediately
//	_journal_mode  WAL: readers do not block the writer
//	_txlock        BEGIN IMMEDIATE, so a write transaction takes its lock up
//	               front instead of failing when upgrading a read transaction
func dataSourceName(dbPath string) string {
	params := url.Values{}
	params.Set("_foreign_keys", "on")
	params.Set("_busy_timeout", fmt.Sprint(busyTimeoutMs))
	params.Set("_journal_mode", "WAL")
	params.Set("_txlock", "immediate")
	return "file:" + dbPath + "?" + params.Encode()
}

// Ping reports whether the database is reachable.
func (s *Store) Ping(ctx context.Context) error {
	if err := s.db.PingContext(ctx); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}
	return nil
}

// Deck returns the deck repository.
func (s *Store) Deck() *DeckRepository {
	return s.deckRepository
}

// Game returns the game repository.
func (s *Store) Game() *GameRepository {
	return s.gameRepository
}

// Close releases the database handle.
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}
