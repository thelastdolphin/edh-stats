package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mattn/go-sqlite3"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// DeckRepository reads and writes decks.
type DeckRepository struct {
	db *sql.DB
}

// Create inserts the deck and fills in its generated ID.
//
// A violation of the decks_name_owner_unique constraint is reported as
// edherrors.ErrDeckDuplicate so that the HTTP layer can answer 409 without
// knowing anything about SQL.
func (dr *DeckRepository) Create(ctx context.Context, deck *model.Deck) (*model.Deck, error) {
	const query = `INSERT INTO decks (name, owner, type) VALUES (?, ?, ?)`

	result, err := dr.db.ExecContext(ctx, query, deck.Name, deck.Owner, deck.Type)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, fmt.Errorf("insert deck %q/%q: %w", deck.Name, deck.Owner, edherrors.ErrDeckDuplicate)
		}
		return nil, fmt.Errorf("insert deck %q/%q: %w", deck.Name, deck.Owner, err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("deck insert id: %w", err)
	}
	deck.ID = id
	return deck, nil
}

// GetByID returns the deck with the given ID, or edherrors.ErrDeckNotFound.
func (dr *DeckRepository) GetByID(ctx context.Context, id int64) (*model.Deck, error) {
	const query = `SELECT id, name, owner, type FROM decks WHERE id = ?`

	var deck model.Deck
	err := dr.db.QueryRowContext(ctx, query, id).
		Scan(&deck.ID, &deck.Name, &deck.Owner, &deck.Type)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("deck %d: %w", id, edherrors.ErrDeckNotFound)
		}
		return nil, fmt.Errorf("select deck %d: %w", id, err)
	}
	return &deck, nil
}

// List returns the decks matching the filters, ordered by ID and paginated.
func (dr *DeckRepository) List(ctx context.Context, filters DeckFilters) ([]*model.Deck, error) {
	query := `SELECT id, name, owner, type FROM decks WHERE 1=1`
	var args []any

	// The fragments below are constants and the values travel as placeholders,
	// so the concatenation cannot introduce SQL injection.
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

	filters = filters.Clamp()

	// ORDER BY is what makes LIMIT/OFFSET meaningful: without it the database is
	// free to return the same page with different rows on every request.
	query += " ORDER BY id LIMIT ? OFFSET ?"
	args = append(args, filters.Limit, filters.Offset())

	rows, err := dr.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select decks: %w", err)
	}
	defer rows.Close()

	// Start from an empty slice so an empty result is encoded as [] rather than
	// null, which is easier for API clients to consume.
	decks := make([]*model.Deck, 0, filters.Limit)
	for rows.Next() {
		var deck model.Deck
		if err := rows.Scan(&deck.ID, &deck.Name, &deck.Owner, &deck.Type); err != nil {
			return nil, fmt.Errorf("scan deck: %w", err)
		}
		decks = append(decks, &deck)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate decks: %w", err)
	}
	return decks, nil
}

// isUniqueViolation reports whether err is a UNIQUE or PRIMARY KEY constraint
// violation.
//
// The driver's typed error is inspected instead of matching the message text.
// Text matching is what broke the previous implementation: it looked for
// "already exists" while SQLite reports
// "UNIQUE constraint failed: decks.name, decks.owner", so duplicates ended up
// as 500 responses.
func isUniqueViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	switch sqliteErr.ExtendedCode {
	case sqlite3.ErrConstraintUnique, sqlite3.ErrConstraintPrimaryKey:
		return true
	default:
		return false
	}
}
