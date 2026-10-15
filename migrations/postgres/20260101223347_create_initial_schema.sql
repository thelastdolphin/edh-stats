-- Схема PostgreSQL. Сервис сейчас работает на SQLite (migrations/sqlite3), а эта
-- миграция держится на случай перехода и обязана повторять тот же контракт.
-- Если она уже применялась на какой-нибудь базе, правьте её не здесь, а новой
-- миграцией: goose отмечает применённые версии по номеру и тело файла не сверяет.
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

-- Ширина и обязательность полей повторяют контракт API: name, owner и type
-- обязательны и ограничены 50 символами (internal/app/http/dto/deck_request.go).
-- Раньше здесь стояли VARCHAR(200)/VARCHAR(100) и nullable-колонки, то есть схема
-- разрешала то, что сервис отклоняет.
-- Проверка на непустое значение намеренно оставлена валидации API, а не CHECK:
-- так эта схема остаётся совместимой с SQLite-версией, где колонки уже созданы
-- nullable и не могут быть ужесточены без перестройки таблицы.
CREATE TABLE decks (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(50) NOT NULL,
    owner VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,
    -- ВНИМАНИЕ: ЗДЕСЬ ДОБАВЛЯТЬ НОВЫЕ ПОЛЯ ДЛЯ КОЛОД
    -- КОНЕЦ ЗОНЫ ДЛЯ ПОЛЕЙ
    CONSTRAINT decks_name_owner_unique UNIQUE (name, owner) -- Колода уникальна по названию+автору
);

COMMENT ON TABLE decks IS 'Все колоды, участвующие в играх.';
COMMENT ON COLUMN decks.name IS 'Название колоды, 1..50 символов.';
COMMENT ON COLUMN decks.owner IS 'Владелец колоды, 1..50 символов.';
COMMENT ON COLUMN decks.type IS 'Свободный текст (архетип, формат), 1..50 символов: список значений намеренно не ограничен.';

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
    result_type_id SMALLINT NOT NULL REFERENCES result_types(id) ON DELETE RESTRICT, -- Ссылка на тип результата
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
