-- +goose Up
-- +goose StatementBegin

-- PostgreSQL mirror of the SQLite migration of the same version.
--
-- This directory is not embedded into the binary and is not applied by the
-- service; it is kept so the schema of the future backend stays in step with the
-- one that is actually run.

ALTER TABLE game_deck ALTER COLUMN result_type_id DROP NOT NULL;
ALTER TABLE game_deck ADD COLUMN player TEXT;

CREATE INDEX game_deck_deck_id_idx ON game_deck(deck_id);
CREATE INDEX game_deck_player_idx ON game_deck(player);
CREATE INDEX games_played_at_idx ON games(played_at);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS games_played_at_idx;
DROP INDEX IF EXISTS game_deck_player_idx;
DROP INDEX IF EXISTS game_deck_deck_id_idx;

ALTER TABLE game_deck DROP COLUMN player;
DELETE FROM game_deck WHERE result_type_id IS NULL;
ALTER TABLE game_deck ALTER COLUMN result_type_id SET NOT NULL;

-- +goose StatementEnd
