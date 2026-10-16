package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

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

// Update stores the name, owner and type of the deck identified by its ID.
//
// Renaming a deck onto the name and owner of another one is a violation of
// decks_name_owner_unique, and is reported as edherrors.ErrDeckDuplicate so that
// the HTTP layer can answer 409, exactly as Create does.
func (dr *DeckRepository) Update(ctx context.Context, deck *model.Deck) error {
	const query = `UPDATE decks SET name = ?, owner = ?, type = ? WHERE id = ?`

	result, err := dr.db.ExecContext(ctx, query, deck.Name, deck.Owner, deck.Type, deck.ID)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("update deck %d to %q/%q: %w", deck.ID, deck.Name, deck.Owner, edherrors.ErrDeckDuplicate)
		}
		return fmt.Errorf("update deck %d: %w", deck.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update deck %d: %w", deck.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update deck %d: %w", deck.ID, edherrors.ErrDeckNotFound)
	}
	return nil
}

// Delete removes the deck with the given ID.
//
// A deck that a game still refers to is not removed: the schema declares
// ON DELETE RESTRICT for it, and that is reported as edherrors.ErrDeckInUse
// rather than as a driver error.
func (dr *DeckRepository) Delete(ctx context.Context, id int64) error {
	const query = `DELETE FROM decks WHERE id = ?`

	result, err := dr.db.ExecContext(ctx, query, id)
	if err != nil {
		if isForeignKeyViolation(err) {
			return fmt.Errorf("delete deck %d: %w", id, edherrors.ErrDeckInUse)
		}
		return fmt.Errorf("delete deck %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete deck %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete deck %d: %w", id, edherrors.ErrDeckNotFound)
	}
	return nil
}

// List returns the decks matching the filters, ordered by ID and paginated.
func (dr *DeckRepository) List(ctx context.Context, filters DeckFilters) ([]*model.Deck, error) {
	conditions, args := deckConditions(filters)

	filters = filters.Clamp()

	// ORDER BY is what makes LIMIT/OFFSET meaningful: without it the database is
	// free to return the same page with different rows on every request.
	query := `SELECT id, name, owner, type FROM decks WHERE 1=1` + conditions +
		` ORDER BY id LIMIT ? OFFSET ?`
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

// Count returns how many decks match the filters, ignoring pagination.
func (dr *DeckRepository) Count(ctx context.Context, filters DeckFilters) (int, error) {
	conditions, args := deckConditions(filters)

	var count int
	if err := dr.db.QueryRowContext(ctx,
		`SELECT count(*) FROM decks WHERE 1=1`+conditions, args...).Scan(&count); err != nil {
		return 0, fmt.Errorf("count decks: %w", err)
	}
	return count, nil
}

// deckConditions builds the WHERE clause shared by List and Count.
//
// The fragments are constants and the values travel as placeholders, so the
// concatenation cannot introduce SQL injection.
func deckConditions(filters DeckFilters) (string, []any) {
	conditions := ""
	var args []any

	if filters.Name != "" {
		conditions += " AND name = ?"
		args = append(args, filters.Name)
	}
	if filters.Owner != "" {
		conditions += " AND owner = ?"
		args = append(args, filters.Owner)
	}
	if filters.Type != "" {
		conditions += " AND type = ?"
		args = append(args, filters.Type)
	}
	if filters.Search != "" {
		// LIKE ignores case for ASCII, which is what deck names are written in; for
		// other alphabets it compares exactly.
		pattern := likePattern(filters.Search)
		conditions += ` AND (name LIKE ? ESCAPE '\' OR owner LIKE ? ESCAPE '\')`
		args = append(args, pattern, pattern)
	}

	return conditions, args
}

// likePattern turns a search term into a LIKE pattern.
//
// The wildcards of the term are escaped, so that searching for "100%" looks for that
// text instead of matching every deck, and searching for "_" does not match the whole
// table.
func likePattern(term string) string {
	escaper := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + escaper.Replace(term) + "%"
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

// isForeignKeyViolation reports whether err is the constraint failure SQLite
// raises when a deck is still referenced by a game.
//
// The primary code is checked, not the extended one, and that is deliberate: for
// ON DELETE RESTRICT SQLite reports SQLITE_CONSTRAINT_TRIGGER (1811) rather than
// SQLITE_CONSTRAINT_FOREIGNKEY (787), because the restriction is implemented as an
// internal trigger - even though the message reads "FOREIGN KEY constraint
// failed". A DELETE on decks can only break that one constraint, so
// SQLITE_CONSTRAINT is both accurate and independent of which variant the driver
// happens to surface.
func isForeignKeyViolation(err error) bool {
	var sqliteErr sqlite3.Error
	if !errors.As(err, &sqliteErr) {
		return false
	}
	return sqliteErr.Code == sqlite3.ErrConstraint
}
