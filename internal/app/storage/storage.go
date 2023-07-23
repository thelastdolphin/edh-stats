package storage

import (
	"context"
)

type pgxIface interface {
	QueryRow(context.Context, string, ...interface{}) pgx.Row
	Close(ctx context.Context) error
}

type Store struct {
	dbConfig         *string
	db               pgxIface
	playerRepository *PlayerRepository
	deckRepository   *DeckRepository
}

func (s *Store) Player() *PlayerRepository {
	if s.playerRepository == nil {
		s.playerRepository = &PlayerRepository{}
	}
	return s.playerRepository
}

func (s *Store) Deck() *DeckRepository {
	if s.deckRepository == nil {
		s.deckRepository = &DeckRepository{}
	}
	return s.deckRepository
}
