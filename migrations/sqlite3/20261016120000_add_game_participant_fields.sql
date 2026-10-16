-- +goose Up
-- +goose StatementBegin

-- game_deck gains the two things the game record needs: a participation is
-- optional in its result, and it knows who piloted the deck.
--
--   * result_type_id loses NOT NULL: the result is only known for the deck of the
--     person keeping these statistics. In a pod of four only one player wins, so
--     filling in a result for everybody else would be inventing data.
--   * player names the pilot of the deck in this game. It is deliberately not a
--     column on decks: a deck can be lent out and played by somebody else, so the
--     pilot belongs to the participation, not to the deck.
--
-- SQLite cannot drop a NOT NULL constraint in place, so the table is rebuilt with
-- the documented create/copy/drop/rename sequence. It runs inside goose's
-- transaction and needs no PRAGMA foreign_keys=OFF: nothing references game_deck,
-- so dropping it breaks no reference, and every copied row satisfies the
-- constraints the new table declares.

CREATE TABLE game_deck_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    deck_id INTEGER NOT NULL REFERENCES decks(id) ON DELETE RESTRICT,
    result_type_id INTEGER REFERENCES result_types(id),
    player TEXT,
    win_description TEXT,
    CONSTRAINT game_deck_game_deck_unique UNIQUE (game_id, deck_id)
);

-- The ids are copied so that anything referring to a participation keeps working
-- after the rebuild.
INSERT INTO game_deck_new (id, game_id, deck_id, result_type_id, player, win_description)
    SELECT id, game_id, deck_id, result_type_id, NULL, win_description FROM game_deck;

DROP TABLE game_deck;
ALTER TABLE game_deck_new RENAME TO game_deck;

-- Indexes for the game statistics. game_id is left out on purpose: it is the
-- leading column of the UNIQUE constraint above, so it is already indexed.
CREATE INDEX game_deck_deck_id_idx ON game_deck(deck_id);
CREATE INDEX game_deck_player_idx ON game_deck(player);
CREATE INDEX games_played_at_idx ON games(played_at);

-- +goose StatementEnd

-- No Down section on purpose. The reverse would have to drop the participations
-- whose result was never recorded, and it would silently lose the player column;
-- restoring a backup is the honest way to go back.
