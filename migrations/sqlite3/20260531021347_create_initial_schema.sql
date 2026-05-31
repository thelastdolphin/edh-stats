-- +goose Up
-- +goose StatementBegin

-- Таблица типов результатов
CREATE TABLE result_types (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

-- Начальные данные для типов результатов
INSERT INTO result_types (name) VALUES
('win'),
('loss'),
('draw');

-- Таблица колод
CREATE TABLE decks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    owner TEXT,
    type TEXT,
    CONSTRAINT decks_name_owner_unique UNIQUE (name, owner)
);

-- Таблица игр
CREATE TABLE games (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    played_at DATETIME DEFAULT CURRENT_TIMESTAMP NOT NULL,
    notes TEXT
);

-- Связующая таблица игра-колода
CREATE TABLE game_deck (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    game_id INTEGER NOT NULL REFERENCES games(id) ON DELETE CASCADE,
    deck_id INTEGER NOT NULL REFERENCES decks(id) ON DELETE RESTRICT,
    result_type_id INTEGER NOT NULL REFERENCES result_types(id),
    win_description TEXT,
    CONSTRAINT game_deck_game_deck_unique UNIQUE (game_id, deck_id)
);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP TABLE IF EXISTS game_deck;
DROP TABLE IF EXISTS games;
DROP TABLE IF EXISTS decks;
DROP TABLE IF EXISTS result_types;

-- +goose StatementEnd