package dto

import (
	"strings"
	"testing"
)

func validRequest() CreateGameRequest {
	return CreateGameRequest{
		PlayedAt: "2026-02-07",
		Notes:    "opps conceded",
		Decks: []GameDeckRequest{
			{DeckID: 1, Player: "Andrew", Result: ResultWin},
			{DeckID: 2},
		},
	}
}

func TestCreateGameRequest_Valid(t *testing.T) {
	if err := validRequest().Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// TestCreateGameRequest_PlayedAtIsOptional covers the column default: a request
// without a date is stored with the current one.
func TestCreateGameRequest_PlayedAtIsOptional(t *testing.T) {
	request := validRequest()
	request.PlayedAt = ""

	if err := request.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

func TestCreateGameRequest_Normalize(t *testing.T) {
	request := CreateGameRequest{
		PlayedAt: "  2026-02-07  ",
		Notes:    "  a note  ",
		Decks: []GameDeckRequest{
			{DeckID: 1, Player: "  Andrew  ", Result: "  WIN "},
			{DeckID: 2},
		},
	}

	request.Normalize()

	if request.PlayedAt != "2026-02-07" || request.Notes != "a note" {
		t.Errorf("normalized request = %+v", request)
	}
	if request.Decks[0].Player != "Andrew" || request.Decks[0].Result != ResultWin {
		t.Errorf("normalized participant = %+v", request.Decks[0])
	}
	if err := request.Validate(); err != nil {
		t.Errorf("Validate after Normalize: %v", err)
	}
}

func TestCreateGameRequest_ValidateRejects(t *testing.T) {
	for name, mutate := range map[string]func(*CreateGameRequest){
		"impossible date":           func(r *CreateGameRequest) { r.PlayedAt = "2026-13-01" },
		"date with the wrong shape": func(r *CreateGameRequest) { r.PlayedAt = "07.02.26" },
		"single deck":               func(r *CreateGameRequest) { r.Decks = r.Decks[:1] },
		"no decks":                  func(r *CreateGameRequest) { r.Decks = nil },
		"too many decks": func(r *CreateGameRequest) {
			decks := make([]GameDeckRequest, MaxParticipants+1)
			for i := range decks {
				decks[i] = GameDeckRequest{DeckID: int64(i + 1)}
			}
			r.Decks = decks
		},
		"deck id zero":     func(r *CreateGameRequest) { r.Decks[0].DeckID = 0 },
		"deck id negative": func(r *CreateGameRequest) { r.Decks[0].DeckID = -3 },
		"the same deck twice": func(r *CreateGameRequest) {
			r.Decks[1].DeckID = r.Decks[0].DeckID
		},
		"unknown result":  func(r *CreateGameRequest) { r.Decks[0].Result = "victory" },
		"player too long": func(r *CreateGameRequest) { r.Decks[0].Player = strings.Repeat("a", MaxPlayerLen+1) },
		"notes too long":  func(r *CreateGameRequest) { r.Notes = strings.Repeat("a", MaxNotesLen+1) },
	} {
		t.Run(name, func(t *testing.T) {
			request := validRequest()
			mutate(&request)

			if err := request.Validate(); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestCreateGameRequest_AcceptsEveryResultName(t *testing.T) {
	for _, result := range []string{ResultWin, ResultLoss, ResultDraw, ""} {
		request := validRequest()
		request.Decks[0].Result = result

		if err := request.Validate(); err != nil {
			t.Errorf("result %q: %v", result, err)
		}
	}
}
