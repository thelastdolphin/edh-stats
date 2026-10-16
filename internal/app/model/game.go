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
//
// DeckName and DeckOwner are read from the deck the participation refers to. They are
// filled in whenever a game is read or listed, and ignored when one is written, so
// that a client showing a game does not have to fetch the whole deck list just to put
// names next to the identifiers.
type GameDeck struct {
	DeckID         int64  `json:"deck_id"`
	DeckName       string `json:"deck_name,omitempty"`
	DeckOwner      string `json:"deck_owner,omitempty"`
	Player         string `json:"player,omitempty"`
	ResultTypeID   *int64 `json:"result_type_id,omitempty"`
	WinDescription string `json:"win_description,omitempty"`
}
