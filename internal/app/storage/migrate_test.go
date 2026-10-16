package storage

import (
	"context"
	"database/sql"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/pressly/goose/v3"

	"github.com/thelastdolphin/edh-stats/migrations"
)

// initialSchemaVersion is the version of the first migration. Migrating up to it
// reproduces the schema of the release before game_deck was rebuilt.
const initialSchemaVersion = 20260531021347

// TestMigrate_UpgradesPopulatedDatabase covers the one migration that rewrites a
// table rather than altering it.
//
// SQLite cannot drop a NOT NULL constraint in place, so game_deck is rebuilt, and
// the service migrates itself on startup - which means this code runs against a
// real database with data in it. The test therefore builds the old schema, fills
// it the way the previous release would have, and only then migrates forward.
func TestMigrate_UpgradesPopulatedDatabase(t *testing.T) {
	ctx := context.Background()

	db, err := sql.Open("sqlite3", dataSourceName(filepath.Join(t.TempDir(), "legacy.db")))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	goose.SetBaseFS(migrations.FS)
	if err := goose.SetDialect(dialect); err != nil {
		t.Fatalf("set goose dialect: %v", err)
	}
	if err := goose.UpToContext(ctx, db, dialect, initialSchemaVersion); err != nil {
		t.Fatalf("migrate to the initial schema: %v", err)
	}

	// The old shape of the data: two decks, one game, and a result for every
	// participation, because the column did not allow anything else.
	legacy := []string{
		`INSERT INTO decks (name, owner, type) VALUES ('Legacy Deck', 'Andrew', 'control')`,
		`INSERT INTO decks (name, owner, type) VALUES ('Opponent Deck', NULL, NULL)`,
		`INSERT INTO games (played_at, notes) VALUES ('2026-02-07', 'legacy game')`,
		`INSERT INTO game_deck (game_id, deck_id, result_type_id, win_description) VALUES (1, 1, 1, 'won by combo')`,
		`INSERT INTO game_deck (game_id, deck_id, result_type_id, win_description) VALUES (1, 2, 2, NULL)`,
	}
	for _, statement := range legacy {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			t.Fatalf("seed %q: %v", statement, err)
		}
	}

	if err := goose.UpContext(ctx, db, dialect); err != nil {
		t.Fatalf("apply the remaining migrations: %v", err)
	}

	type participation struct {
		id, gameID, deckID int64
		result             sql.NullInt64
		player             sql.NullString
		winDescription     sql.NullString
	}

	readParticipations := func(t *testing.T) []participation {
		t.Helper()

		rows, err := db.QueryContext(ctx,
			`SELECT id, game_id, deck_id, result_type_id, player, win_description
			   FROM game_deck ORDER BY id`)
		if err != nil {
			t.Fatalf("select participations: %v", err)
		}
		defer rows.Close()

		var got []participation
		for rows.Next() {
			var p participation
			if err := rows.Scan(&p.id, &p.gameID, &p.deckID, &p.result, &p.player, &p.winDescription); err != nil {
				t.Fatalf("scan participation: %v", err)
			}
			got = append(got, p)
		}
		if err := rows.Err(); err != nil {
			t.Fatalf("iterate participations: %v", err)
		}
		return got
	}

	t.Run("existing participations survive unchanged", func(t *testing.T) {
		want := []participation{
			{
				id: 1, gameID: 1, deckID: 1,
				result:         sql.NullInt64{Int64: 1, Valid: true},
				winDescription: sql.NullString{String: "won by combo", Valid: true},
			},
			{
				id: 2, gameID: 1, deckID: 2,
				result: sql.NullInt64{Int64: 2, Valid: true},
			},
		}
		if got := readParticipations(t); !reflect.DeepEqual(got, want) {
			t.Errorf("participations after the migration = %+v, want %+v", got, want)
		}
	})

	t.Run("a result is optional and a player can be recorded", func(t *testing.T) {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO decks (name, owner, type) VALUES ('Third Deck', NULL, NULL)`); err != nil {
			t.Fatalf("insert deck: %v", err)
		}
		if _, err := db.ExecContext(ctx,
			`INSERT INTO games (played_at) VALUES ('2026-10-10')`); err != nil {
			t.Fatalf("insert game: %v", err)
		}

		result, err := db.ExecContext(ctx,
			`INSERT INTO game_deck (game_id, deck_id, result_type_id, player) VALUES (2, 3, NULL, 'Andrew')`)
		if err != nil {
			t.Fatalf("insert participation without a result: %v", err)
		}

		// sqlite_sequence has to follow the rename, otherwise AUTOINCREMENT hands
		// out an id that is already taken.
		id, err := result.LastInsertId()
		if err != nil {
			t.Fatalf("last insert id: %v", err)
		}
		if id != 3 {
			t.Errorf("new participation id = %d, want 3 (the sequence was not carried over by the rename)", id)
		}
	})

	t.Run("one participation per deck and game is still enforced", func(t *testing.T) {
		if _, err := db.ExecContext(ctx,
			`INSERT INTO game_deck (game_id, deck_id, result_type_id) VALUES (2, 3, 1)`); err == nil {
			t.Error("a second participation for the same deck and game was accepted")
		}
	})

	t.Run("foreign keys are still enforced", func(t *testing.T) {
		var enabled int
		if err := db.QueryRowContext(ctx, "PRAGMA foreign_keys").Scan(&enabled); err != nil {
			t.Fatalf("read pragma: %v", err)
		}
		if enabled != 1 {
			t.Fatalf("PRAGMA foreign_keys = %d, want 1", enabled)
		}

		if _, err := db.ExecContext(ctx,
			`INSERT INTO game_deck (game_id, deck_id, result_type_id) VALUES (999, 1, 1)`); err == nil {
			t.Error("a participation referring to a missing game was accepted")
		}
	})

	t.Run("the indexes the statistics need exist", func(t *testing.T) {
		for _, name := range []string{"game_deck_deck_id_idx", "game_deck_player_idx", "games_played_at_idx"} {
			var found string
			err := db.QueryRowContext(ctx,
				`SELECT name FROM sqlite_master WHERE type = 'index' AND name = ?`, name).Scan(&found)
			if err != nil {
				t.Errorf("index %s: %v", name, err)
			}
		}
	})
}
