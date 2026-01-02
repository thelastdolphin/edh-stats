package storage

import (
	"context"

	"example.com/edh-stats/internal/app/model"
	"github.com/jackc/pgx/v4/pgxpool"
)

type DeckRepository struct {
	db *pgxpool.Pool
}

func (dr *DeckRepository) Create(ctx context.Context, d *model.Deck) (*model.Deck, error) {
	if err := dr.db.QueryRow(
		ctx,
		"INSERT INTO decks (name, owner, type) VALUES ($1, $2, $3) RETURNING id",
		d.Name,
		d.Owner,
		d.Type,
	).Scan(&d.ID); err != nil {
		return nil, err
	}
	return d, nil
}

func (dr *DeckRepository) FindById(ctx context.Context, id int) (*model.Deck, error) {
	deck := &model.Deck{}
	if err := dr.db.QueryRow(
		ctx,
		"SELECT id, name, owner, type FROM decks WHERE id = $1",
		id,
	).Scan(&deck.ID, &deck.Name, &deck.Owner, &deck.Type); err != nil {
		return nil, err
	}
	return deck, nil
}
