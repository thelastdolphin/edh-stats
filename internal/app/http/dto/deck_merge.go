package dto

import "errors"

// MergeDeckRequest is the JSON body of POST /decks/{id}/merge.
//
// Into is required. A merge that does not say which deck survives cannot be guessed: the
// guess would delete one of the two, and which one is the caller's decision, not the
// server's.
type MergeDeckRequest struct {
	Into *int64 `json:"into"`
}

// Validate reports the first problem with the payload.
func (r MergeDeckRequest) Validate() error {
	if r.Into == nil {
		return errors.New("into is required")
	}
	if *r.Into < 1 {
		return errors.New("into must be a positive integer")
	}
	return nil
}

// MergeDeckResponse reports how many games moved to the surviving deck.
type MergeDeckResponse struct {
	Moved int `json:"moved"`
}
