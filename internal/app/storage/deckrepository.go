package storage

import (
	"context"
	"fmt"

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

func (dr *DeckRepository) GetById(ctx context.Context, id int) (*model.Deck, error) {
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

func (dr *DeckRepository) List(ctx context.Context, filters DeckFilters) ([]*model.Deck, error) {
	query := "SELECT id, name, owner, type FROM decks WHERE 1=1"
	var args []interface{}
	argPos := 1

	if filters.Name != "" {
		query += fmt.Sprintf(" AND name = $%d", argPos)
		args = append(args, filters.Name)
		argPos++
	}
	if filters.Owner != "" {
		query += fmt.Sprintf(" AND owner = $%d", argPos)
		args = append(args, filters.Owner)
		argPos++
	}
	if filters.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argPos)
		args = append(args, filters.Type)
		argPos++
	}

	//query += fmt.Sprintf(" LIMIT %d OFFSET %d", argPos, argPos+1)
	//args = append(args, filters.Limit, (filters.Page-1)*filters.Limit)

	rows, err := dr.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var decks []*model.Deck

	for rows.Next() {
		var deck model.Deck
		if err := rows.Scan(&deck.ID, &deck.Name, &deck.Owner, &deck.Type); err != nil {
			return nil, err
		}
		decks = append(decks, &deck)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return decks, nil
}
