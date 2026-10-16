package model

// DeckStats is one row of the deck statistics: how often a deck was played and how
// it did.
//
// Wins, losses and draws count only the games in which a result was recorded, so
// they can add up to less than Games. The API deliberately does not send a win rate:
// a deck with no recorded result would come out as 0 of 0, which reads as "never
// won". The client decides what to display when nothing was recorded.
type DeckStats struct {
	DeckID int64  `json:"deck_id"`
	Name   string `json:"name"`
	Owner  string `json:"owner"`
	Games  int    `json:"games"`
	Wins   int    `json:"wins"`
	Losses int    `json:"losses"`
	Draws  int    `json:"draws"`
}
