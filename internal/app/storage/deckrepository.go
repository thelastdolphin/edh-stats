package storage

import (
	"context"
	"database/sql"
	"fmt"

	"example.com/edh-stats/internal/app/model"
)

type DeckRepository struct {
	db *sql.DB
}

func (dr *DeckRepository) Create(ctx context.Context, d *model.Deck) (*model.Deck, error) {
	query := `INSERT INTO decks (name, owner, type) VALUES (?, ?, ?)`
	result, err := dr.db.ExecContext(ctx, query, d.Name, d.Owner, d.Type)
	if err != nil {
		return nil, fmt.Errorf("failed inserting into db %v", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get id %v", err)
	}
	d.ID = id
	return d, nil
}

func (dr *DeckRepository) GetById(ctx context.Context, id int) (*model.Deck, error) {
	deck := &model.Deck{}
	query := `SELECT id, name, owner, type FROM decks where id = ?`
	err := dr.db.QueryRowContext(ctx, query, id).Scan(&deck.ID, &deck.Name, &deck.Owner, &deck.Type)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("deck not found")
		}
		return nil, fmt.Errorf("failed to get deck by id %v ", err)
	}
	return deck, nil
}

func (dr *DeckRepository) List(ctx context.Context, filters DeckFilters) ([]*model.Deck, error) {
	query := "SELECT id, name, owner, type FROM decks WHERE 1=1"
	var args []interface{}

	if filters.Name != "" {
		query += " AND name = ?"
		args = append(args, filters.Name)
	}
	if filters.Owner != "" {
		query += " AND owner = ?"
		args = append(args, filters.Owner)
	}
	if filters.Type != "" {
		query += " AND type = ?"
		args = append(args, filters.Type)
	}

	// Пагинация (если понадобится)
	// if filters.Limit > 0 {
	//     query += " LIMIT ? OFFSET ?"
	//     args = append(args, filters.Limit, filters.Offset)
	// }

	rows, err := dr.db.QueryContext(ctx, query, args...)
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

	return decks, rows.Err()
}
