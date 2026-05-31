package storage

import (
	"context"
	"fmt"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db             *sql.DB
	deckRepository *DeckRepository
}

func NewStore(ctx context.Context, dbPath string) (*Store, error) {

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		return nil, fmt.Errorf("failed to enable foreign keys %v", err)
	}

	store := &Store{db: db}
	store.deckRepository = &DeckRepository{db: db}
	return store, nil
}

func (s *Store) Deck() *DeckRepository {
	return s.deckRepository
}

func (s *Store) Close() {
	if s.db != nil {
		s.db.Close()
	}
}
