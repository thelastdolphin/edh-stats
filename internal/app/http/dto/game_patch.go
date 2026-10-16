package dto

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// PatchGameRequest is the JSON body of PATCH /games/{id}.
//
// The fields are pointers so that "not mentioned" stays distinguishable from "set to
// an empty value": a PATCH changes only what the request carries.
type PatchGameRequest struct {
	PlayedAt *string `json:"played_at"`
	Notes    *string `json:"notes"`
}

// Normalize trims the fields the request carries.
func (r *PatchGameRequest) Normalize() {
	r.PlayedAt = trimmed(r.PlayedAt)
	r.Notes = trimmed(r.Notes)
}

// Validate reports the first problem found in the payload.
func (r PatchGameRequest) Validate() error {
	if r.PlayedAt == nil && r.Notes == nil {
		return errors.New("provide at least one of played_at or notes")
	}

	if r.PlayedAt != nil {
		if _, err := time.Parse(model.DateLayout, *r.PlayedAt); err != nil {
			return errors.New("played_at must be a date in YYYY-MM-DD form")
		}
	}

	if r.Notes != nil && utf8.RuneCountInString(*r.Notes) > MaxNotesLen {
		return fmt.Errorf("notes must be at most %d characters", MaxNotesLen)
	}

	return nil
}

// Apply writes the fields the request carries onto the game.
func (r PatchGameRequest) Apply(game *model.Game) {
	if r.PlayedAt != nil {
		game.PlayedAt = *r.PlayedAt
	}
	if r.Notes != nil {
		game.Notes = *r.Notes
	}
}

// PatchGameDeckRequest is the JSON body of PATCH /games/{id}/decks/{deckID}.
//
// Both fields may be cleared with an empty string, which is the one place in the API
// where empty is a value rather than a validation error: the columns are nullable,
// and a result recorded by mistake has to be removable.
type PatchGameDeckRequest struct {
	Player *string `json:"player"`
	Result *string `json:"result"`
}

// Normalize trims the fields the request carries, and lower-cases the result so that
// "Win" and "win" are the same value.
func (r *PatchGameDeckRequest) Normalize() {
	r.Player = trimmed(r.Player)

	if r.Result != nil {
		result := strings.ToLower(strings.TrimSpace(*r.Result))
		r.Result = &result
	}
}

// Validate reports the first problem found in the payload.
func (r PatchGameDeckRequest) Validate() error {
	if r.Player == nil && r.Result == nil {
		return errors.New("provide at least one of player or result")
	}

	if r.Player != nil && utf8.RuneCountInString(*r.Player) > MaxPlayerLen {
		return fmt.Errorf("player must be at most %d characters", MaxPlayerLen)
	}

	if r.Result != nil {
		switch *r.Result {
		case "", ResultWin, ResultLoss, ResultDraw:
		default:
			return fmt.Errorf("result must be %s, %s, %s, or empty to clear it",
				ResultWin, ResultLoss, ResultDraw)
		}
	}

	return nil
}

// trimmed returns the trimmed value, keeping nil as nil.
func trimmed(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	return &trimmed
}
