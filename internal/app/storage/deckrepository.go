package storage

import (
	"context"
	"example.com/edh-stats/internal/app/model"
)

type DeckRepository struct{}

func (pr *DeckRepository) Create(d *model.Deck, s *Store) (*model.Deck, error) {
	if err := s.db.QueryRow(
		context.Background(),
		"INSERT INTO players (deckName, decklist) VALUES ($1, $2) RETURNING id",
		d.deckName,
		d.decklist,
	).Scan(&d.ID); err != nil {
		return nil, err
	}
	return d, nil
}
