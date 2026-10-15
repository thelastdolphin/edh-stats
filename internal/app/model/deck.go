package model

// Deck is a deck registered by a player.
//
// The JSON tags keep the response field names identical to the request field
// names: without them encoding/json emits "ID"/"Name"/"Owner"/"Type" while the
// API accepts "name"/"owner"/"type".
type Deck struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Owner string `json:"owner"`
	Type  string `json:"type"`
}
