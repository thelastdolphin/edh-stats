package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/thelastdolphin/edh-stats/internal/app/edherrors"
	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// isoDate is the layout of games.played_at. The recorded data is a calendar date,
// and ISO dates also compare correctly as text.
const isoDate = "2006-01-02"

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
		playedAt = time.Now().Format(isoDate)
	}

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

	const insertParticipant = `INSERT INTO game_deck (game_id, deck_id, result_type_id, player, win_description)
	                           VALUES (?, ?, ?, ?, ?)`
	for _, participant := range game.Decks {
		_, err := tx.ExecContext(ctx, insertParticipant, game.ID, participant.DeckID,
			participant.ResultTypeID, nullIfEmpty(participant.Player), nullIfEmpty(participant.WinDescription))
		if err != nil {
			if isUniqueViolation(err) {
				return nil, fmt.Errorf("add deck %d to game %d: %w",
					participant.DeckID, game.ID, edherrors.ErrDeckAlreadyInGame)
			}
			return nil, fmt.Errorf("add deck %d to game %d: %w", participant.DeckID, game.ID, err)
		}
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

	const participantsQuery = `SELECT deck_id, player, result_type_id, win_description
	                             FROM game_deck WHERE game_id = ? ORDER BY id`
	rows, err := gr.db.QueryContext(ctx, participantsQuery, id)
	if err != nil {
		return nil, fmt.Errorf("select participants of game %d: %w", id, err)
	}
	defer rows.Close()

	// A non-nil slice so that an empty result encodes as [] rather than null.
	game.Decks = make([]*model.GameDeck, 0, 4)
	for rows.Next() {
		var (
			participant    model.GameDeck
			player         sql.NullString
			resultTypeID   sql.NullInt64
			winDescription sql.NullString
		)
		if err := rows.Scan(&participant.DeckID, &player, &resultTypeID, &winDescription); err != nil {
			return nil, fmt.Errorf("scan participant of game %d: %w", id, err)
		}

		participant.Player = player.String
		participant.WinDescription = winDescription.String
		if resultTypeID.Valid {
			id := resultTypeID.Int64
			participant.ResultTypeID = &id
		}
		game.Decks = append(game.Decks, &participant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate participants of game %d: %w", id, err)
	}
	return game, nil
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
