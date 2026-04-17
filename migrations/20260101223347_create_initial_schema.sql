-- +goose Up
-- +goose StatementBegin
CREATE TABLE result_types (
    id SMALLSERIAL PRIMARY KEY,
    name VARCHAR(20) NOT NULL UNIQUE
);

INSERT INTO result_types (name) VALUES
    ('win'),
    ('loss'),
    ('draw');

CREATE TABLE decks (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    owner VARCHAR(100),
    type VARCHAR(50),
    -- ВНИМАНИЕ: ЗДЕСЬ ДОБАВЛЯТЬ НОВЫЕ ПОЛЯ ДЛЯ КОЛОД
    -- КОНЕЦ ЗОНЫ ДЛЯ ПОЛЕЙ
    CONSTRAINT decks_name_owner_unique UNIQUE (name, owner) -- Колода уникальна по названию+автору
);

COMMENT ON TABLE decks IS 'Все колоды, участвующие в играх.';

CREATE TABLE games (
    id BIGSERIAL PRIMARY KEY,
    played_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() NOT NULL,
    notes TEXT
);

COMMENT ON COLUMN games.played_at IS 'Дата и время проведения игры. По умолчанию - момент создания записи.';

CREATE TABLE game_deck (
    id BIGSERIAL PRIMARY KEY,
    game_id BIGINT NOT NULL REFERENCES games(id) ON DELETE CASCADE, -- Ссылка на игру
    deck_id BIGINT NOT NULL REFERENCES decks(id) ON DELETE RESTRICT, -- Ссылка на колоду
    result_type_id SMALLINT NOT NULL REFERENCES result_types(id), -- Ссылка на тип результата
    win_description TEXT, -- Описание победы (почему, как)

    CONSTRAINT game_deck_game_deck_unique UNIQUE (game_id, deck_id)
);

COMMENT ON TABLE game_deck IS 'Связь игры и колоды. Хранит результат конкретной колоды в конкретной игре.';
COMMENT ON COLUMN game_deck.win_description IS 'Описание того, как была достигнута победа. Настоятельно рекомендуется заполнять при результате "win".';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS game_deck;
DROP TABLE IF EXISTS games;
DROP TABLE IF EXISTS decks;
DROP TABLE IF EXISTS result_types;
-- +goose StatementEnd
