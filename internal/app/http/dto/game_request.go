package dto

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
)

// Limits of a game payload.
const (
	MaxPlayerLen = 50
	MaxNotesLen  = 500
	// MaxParticipants is generous for a game of Commander, and it keeps one request
	// from inserting an unbounded number of rows in a single transaction.
	MaxParticipants = 16
)

// Result names as the migration seeds them into result_types.
const (
	ResultWin  = "win"
	ResultLoss = "loss"
	ResultDraw = "draw"
)

// CreateGameRequest is the JSON body of POST /games.
//
// The participants are ordered: the first one is whoever is recording the game, so
// the result of the game is stored on their participation.
type CreateGameRequest struct {
	PlayedAt string            `json:"played_at"`
	Notes    string            `json:"notes"`
	Decks    []GameDeckRequest `json:"decks"`
}

// GameDeckRequest is one participant of the request.
type GameDeckRequest struct {
	DeckID int64  `json:"deck_id"`
	Player string `json:"player"`
	Result string `json:"result"`
}

// Normalize trims the payload, and lower-cases the result so that "Win" and "win"
// are the same value.
func (r *CreateGameRequest) Normalize() {
	r.PlayedAt = strings.TrimSpace(r.PlayedAt)
	r.Notes = strings.TrimSpace(r.Notes)
	for i := range r.Decks {
		r.Decks[i].Player = strings.TrimSpace(r.Decks[i].Player)
		r.Decks[i].Result = strings.ToLower(strings.TrimSpace(r.Decks[i].Result))
	}
}

// Validate reports the first problem found in the payload.
//
// An empty played_at is accepted: the column defaults to the current date, and the
// storage layer applies that default.
func (r CreateGameRequest) Validate() error {
	if r.PlayedAt != "" {
		if _, err := time.Parse(model.DateLayout, r.PlayedAt); err != nil {
			return errors.New("played_at must be a date in YYYY-MM-DD form")
		}
	}

	if utf8.RuneCountInString(r.Notes) > MaxNotesLen {
		return fmt.Errorf("notes must be at most %d characters", MaxNotesLen)
	}

	if len(r.Decks) < 2 {
		return errors.New("a game needs at least two decks")
	}
	if len(r.Decks) > MaxParticipants {
		return fmt.Errorf("a game can hold at most %d decks", MaxParticipants)
	}

	seen := make(map[int64]bool, len(r.Decks))
	for i, participant := range r.Decks {
		if participant.DeckID < 1 {
			return fmt.Errorf("decks[%d]: deck_id must be a positive integer", i)
		}
		if seen[participant.DeckID] {
			return fmt.Errorf("decks[%d]: deck %d appears more than once", i, participant.DeckID)
		}
		seen[participant.DeckID] = true

		if utf8.RuneCountInString(participant.Player) > MaxPlayerLen {
			return fmt.Errorf("decks[%d]: player must be at most %d characters", i, MaxPlayerLen)
		}

		switch participant.Result {
		case "", ResultWin, ResultLoss, ResultDraw:
		default:
			return fmt.Errorf("decks[%d]: result must be %s, %s or %s", i, ResultWin, ResultLoss, ResultDraw)
		}
	}

	return nil
}
