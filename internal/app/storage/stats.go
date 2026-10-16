package storage

import (
	"context"
	"fmt"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// DeckStats returns one row per deck that took part in a game, ordered by how often
// it was played.
//
// games counts the participations of the deck; wins, losses and draws count only the
// participations that carry a result. That is why they can add up to less than games:
// a result is recorded for the deck of whoever keeps the statistics, and inventing
// one for the others would be inventing data. A deck without results reports zeroes,
// and the caller decides what that means.
func (gr *GameRepository) DeckStats(ctx context.Context, filters StatsFilters) ([]*model.DeckStats, error) {
	// played_at holds an ISO date, so plain comparisons order correctly and keep
	// games_played_at_idx usable; wrapping the column in date() would not.
	query := `SELECT d.id, d.name, COALESCE(d.owner, ''),
	                 count(gd.game_id) AS games_played,
	                 sum(CASE WHEN rt.name = 'win'  THEN 1 ELSE 0 END) AS wins,
	                 sum(CASE WHEN rt.name = 'loss' THEN 1 ELSE 0 END) AS losses,
	                 sum(CASE WHEN rt.name = 'draw' THEN 1 ELSE 0 END) AS draws
	            FROM decks d
	            JOIN game_deck gd ON gd.deck_id = d.id
	            JOIN games g      ON g.id = gd.game_id
	            LEFT JOIN result_types rt ON rt.id = gd.result_type_id
	           WHERE 1=1`
	var args []any

	if filters.From != "" {
		query += ` AND g.played_at >= ?`
		args = append(args, filters.From)
	}
	if filters.To != "" {
		query += ` AND g.played_at <= ?`
		args = append(args, filters.To)
	}

	// The order is deterministic so that a client can re-sort safely. owner is
	// wrapped in COALESCE above because the column is nullable.
	query += ` GROUP BY d.id, d.name, d.owner ORDER BY games_played DESC, d.id ASC`

	rows, err := gr.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("select deck statistics: %w", err)
	}
	defer rows.Close()

	// A non-nil slice so that an empty result encodes as [] rather than null.
	stats := make([]*model.DeckStats, 0, 16)
	for rows.Next() {
		var row model.DeckStats
		if err := rows.Scan(&row.DeckID, &row.Name, &row.Owner,
			&row.Games, &row.Wins, &row.Losses, &row.Draws); err != nil {
			return nil, fmt.Errorf("scan deck statistics: %w", err)
		}
		stats = append(stats, &row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate deck statistics: %w", err)
	}
	return stats, nil
}
