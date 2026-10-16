package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// GameRepository reads and writes games and their participants.
type GameRepository struct {
	db *sql.DB
}

// Create stores a game together with its participants and fills in its ID.
//
// The whole game is written in one transaction. A game with only some of its
// participants is not a state worth persisting, and leaving that cleanup to the
// caller is how half-imported data appears.
func (gr *GameRepository) Create(ctx context.Context, game *model.Game) (*model.Game, error) {
	playedAt := game.PlayedAt
	if playedAt == "" {
		playedAt = time.Now().Format(model.DateLayout)
	}
	// Keep the game in step with the row that is about to be written: without this
	// the caller - and therefore the response - would still carry the empty date.
	game.PlayedAt = playedAt

	tx, err := gr.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin game transaction: %w", err)
	}
	// A rollback after a successful commit is a no-op, so this covers every early
	// return below.
	defer func() { _ = tx.Rollback() }()

	result, err := tx.ExecContext(ctx,
		`INSERT INTO games (played_at, notes) VALUES (?, ?)`, playedAt, nullIfEmpty(game.Notes))
	if err != nil {
		return nil, fmt.Errorf("insert game: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("game insert id: %w", err)
	}
	game.ID = id

	if _, err := insertParticipants(ctx, tx, game); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit game transaction: %w", err)
	}
	return game, nil
}

// GetByID returns the game with its participants, in the order they were recorded.
func (gr *GameRepository) GetByID(ctx context.Context, id int64) (*model.Game, error) {
	// date() is doing real work here: rows can hold a full timestamp (the column
	// defaults to CURRENT_TIMESTAMP), the driver hands those back as a time.Time
	// because the column is declared DATETIME, and the model wants the day. A
	// computed column has no declared type, so what arrives is plain text.
	const gameQuery = `SELECT id, date(played_at), notes FROM games WHERE id = ?`

	game := &model.Game{}
	var notes sql.NullString
	err := gr.db.QueryRowContext(ctx, gameQuery, id).Scan(&game.ID, &game.PlayedAt, &notes)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("game %d: %w", id, edherrors.ErrGameNotFound)
		}
		return nil, fmt.Errorf("select game %d: %w", id, err)
	}
	game.Notes = notes.String

	game.Decks = make([]*model.GameDeck, 0, 4)
	if err := gr.loadParticipants(ctx, []*model.Game{game}); err != nil {
		return nil, err
	}
	return game, nil
}

// Update stores the date and the notes of an existing game.
//
// The participants are not touched: they have UpdateParticipant of their own, which
// is what keeps a corrected result from wiping the others.
func (gr *GameRepository) Update(ctx context.Context, game *model.Game) error {
	const query = `UPDATE games SET played_at = ?, notes = ? WHERE id = ?`

	result, err := gr.db.ExecContext(ctx, query, game.PlayedAt, nullIfEmpty(game.Notes), game.ID)
	if err != nil {
		return fmt.Errorf("update game %d: %w", game.ID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update game %d: %w", game.ID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update game %d: %w", game.ID, edherrors.ErrGameNotFound)
	}
	return nil
}

// UpdateParticipant changes who piloted a deck in a game and with what result.
//
// resultTypeID is a pointer because the column is nullable: nil clears the result,
// which is the normal state of every participation except the one belonging to the
// player keeping the statistics.
func (gr *GameRepository) UpdateParticipant(ctx context.Context, gameID, deckID int64, player string, resultTypeID *int64) error {
	const query = `UPDATE game_deck SET player = ?, result_type_id = ? WHERE game_id = ? AND deck_id = ?`

	result, err := gr.db.ExecContext(ctx, query, nullIfEmpty(player), resultTypeID, gameID, deckID)
	if err != nil {
		return fmt.Errorf("update deck %d in game %d: %w", deckID, gameID, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("update deck %d in game %d: %w", deckID, gameID, err)
	}
	if affected == 0 {
		return fmt.Errorf("update deck %d in game %d: %w", deckID, gameID, edherrors.ErrParticipantNotFound)
	}
	return nil
}

// List returns the games matching the filters, newest first, with their
// participants already loaded.
func (gr *GameRepository) List(ctx context.Context, filters GameFilters) ([]*model.Game, error) {
	query := `SELECT id, date(played_at), notes FROM games WHERE 1=1`
	var args []any

	// The fragments below are constants and the values travel as placeholders,
	// so the concatenation cannot introduce SQL injection.
	if filters.DeckID > 0 {
		query += ` AND id IN (SELECT game_id FROM game_deck WHERE deck_id = ?)`
		args = append(args, filters.DeckID)
	}
	if filters.Player != "" {
		query += ` AND id IN (SELECT game_id FROM game_deck WHERE player = ?)`
		args = append(args, filters.Player)
	}
	// played_at holds an ISO date, which compares correctly as text, so the bounds
	// are plain comparisons: wrapping the column in date() would stop SQLite from
	// using games_played_at_idx.
	if filters.From != "" {
		query += ` AND played_at >= ?`
		args = append(args, filters.From)
	}
	if filters.To != "" {
		query += ` AND played_at <= ?`
		args = append(args, filters.To)
	}

	filters = filters.Clamp()

	// Newest first: a listing is read from the most recent game backwards, and the
	// played_at index serves this order.
	query += ` ORDER BY played_at DESC, id DESC LIMIT ? OFFSET ?`
	args = append(args, filters.Limit, filters.Offset())

	rows, err := gr.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select games: %w", err)
	}
	defer rows.Close()

	// Start from an empty slice so an empty result is encoded as [] rather than null.
	games := make([]*model.Game, 0, filters.Limit)
	for rows.Next() {
		game := &model.Game{}
		var notes sql.NullString
		if err := rows.Scan(&game.ID, &game.PlayedAt, &notes); err != nil {
			return nil, fmt.Errorf("scan game: %w", err)
		}
		game.Notes = notes.String
		game.Decks = make([]*model.GameDeck, 0, 4)
		games = append(games, game)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate games: %w", err)
	}

	if err := gr.loadParticipants(ctx, games); err != nil {
		return nil, err
	}
	return games, nil
}

// loadParticipants fills in the participants of the given games with a single
// query, instead of one query per game.
func (gr *GameRepository) loadParticipants(ctx context.Context, games []*model.Game) error {
	if len(games) == 0 {
		return nil
	}

	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(games)), ",")
	query := `SELECT game_id, deck_id, player, result_type_id, win_description
	            FROM game_deck WHERE game_id IN (` + placeholders + `) ORDER BY id`

	args := make([]any, 0, len(games))
	byID := make(map[int64]*model.Game, len(games))
	for _, game := range games {
		args = append(args, game.ID)
		byID[game.ID] = game
	}

	rows, err := gr.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("select participants: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			gameID         int64
			participant    model.GameDeck
			player         sql.NullString
			resultTypeID   sql.NullInt64
			winDescription sql.NullString
		)
		if err := rows.Scan(&gameID, &participant.DeckID, &player, &resultTypeID, &winDescription); err != nil {
			return fmt.Errorf("scan participant: %w", err)
		}

		participant.Player = player.String
		participant.WinDescription = winDescription.String
		if resultTypeID.Valid {
			id := resultTypeID.Int64
			participant.ResultTypeID = &id
		}

		game, ok := byID[gameID]
		if !ok {
			return fmt.Errorf("participant of game %d selected for another game", gameID)
		}
		game.Decks = append(game.Decks, &participant)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate participants: %w", err)
	}
	return nil
}

// insertParticipants writes the participants of a game inside the caller's
// transaction, so that Create and any later rewrite share one implementation.
func insertParticipants(ctx context.Context, tx *sql.Tx, game *model.Game) (int, error) {
	const query = `INSERT INTO game_deck (game_id, deck_id, result_type_id, player, win_description)
	               VALUES (?, ?, ?, ?, ?)`

	for _, participant := range game.Decks {
		_, err := tx.ExecContext(ctx, query, game.ID, participant.DeckID,
			participant.ResultTypeID, nullIfEmpty(participant.Player), nullIfEmpty(participant.WinDescription))
		if err != nil {
			switch {
			case isUniqueViolation(err):
				return 0, fmt.Errorf("add deck %d to game %d: %w",
					participant.DeckID, game.ID, edherrors.ErrDeckAlreadyInGame)
			case isForeignKeyViolation(err):
				// The game was inserted by this transaction, so the only foreign key
				// a participation can break is deck_id.
				return 0, fmt.Errorf("add deck %d to game %d: %w",
					participant.DeckID, game.ID, edherrors.ErrDeckNotFound)
			default:
				return 0, fmt.Errorf("add deck %d to game %d: %w", participant.DeckID, game.ID, err)
			}
		}
	}
	return len(game.Decks), nil
}

// Count returns how many games are stored.
func (gr *GameRepository) Count(ctx context.Context) (int, error) {
	var count int
	if err := gr.db.QueryRowContext(ctx, `SELECT count(*) FROM games`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count games: %w", err)
	}
	return count, nil
}

// ResultTypeIDs returns the result type names mapped to their IDs.
//
// The names are seeded by the migration; looking them up instead of hardcoding the
// IDs keeps the code independent of the order they were inserted in.
func (gr *GameRepository) ResultTypeIDs(ctx context.Context) (map[string]int64, error) {
	rows, err := gr.db.QueryContext(ctx, `SELECT id, name FROM result_types`)
	if err != nil {
		return nil, fmt.Errorf("select result types: %w", err)
	}
	defer rows.Close()

	ids := make(map[string]int64, 3)
	for rows.Next() {
		var (
			id   int64
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, fmt.Errorf("scan result type: %w", err)
		}
		ids[name] = id
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate result types: %w", err)
	}
	return ids, nil
}

// Delete removes the game; its participants go with it through ON DELETE CASCADE.
func (gr *GameRepository) Delete(ctx context.Context, id int64) error {
	result, err := gr.db.ExecContext(ctx, `DELETE FROM games WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete game %d: %w", id, err)
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete game %d: %w", id, err)
	}
	if affected == 0 {
		return fmt.Errorf("delete game %d: %w", id, edherrors.ErrGameNotFound)
	}
	return nil
}

// nullIfEmpty stores an absent value as NULL instead of an empty string, so that
// "not recorded" stays distinguishable from a deliberately empty value.
func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
