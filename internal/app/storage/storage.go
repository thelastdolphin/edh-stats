package storage

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Store struct {
	db             *pgxpool.Pool
	deckRepository *DeckRepository
}

func NewStore(ctx context.Context, connString string) (*Store, error) {

	db, err := pgxpool.Connect(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("error connecting to database: %w", err)
	}

	store := &Store{
		db: db,
	}

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
