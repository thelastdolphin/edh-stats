package storage

import (
	"context"
	"github.com/jackc/pgx/v4"
	"github.com/pkg/errors"
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

type DbMethods struct{}

func (d *DbMethods) QueryRow(ctx context.Context, s string) pgx.Row {
	r := QueryRow(ctx, s)
	return r
}

func (d *DbMethods) Close(iface pgxIface, ctx context.Context) error {
	err := iface.Close(ctx)
	if err != nil {
		return errors.Cause(err)
	}
	return nil
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
