package model

// Game is one recorded game.
//
// PlayedAt is an ISO 8601 calendar date ("2026-02-07") rather than a time.Time:
// the recorded data is a date, SQLite stores it as text, ISO dates sort correctly
// as text, and the type avoids dragging time-zone handling into a schema that has
// no notion of a clock.
type Game struct {
	ID       int64       `json:"id"`
	PlayedAt string      `json:"played_at"`
	Notes    string      `json:"notes"`
	Decks    []*GameDeck `json:"decks"`
}

// GameDeck is one participation in a game: which deck was played, by whom, and -
// only for the deck of the player keeping these statistics - with what result.
//
// The player belongs here rather than on the deck: the same deck can be lent out
// and piloted by somebody else, and pointing at the participation also answers
// which of the participants is the owner of the statistics.
type GameDeck struct {
	DeckID         int64  `json:"deck_id"`
	Player         string `json:"player,omitempty"`
	ResultTypeID   *int64 `json:"result_type_id,omitempty"`
	WinDescription string `json:"win_description,omitempty"`
}
