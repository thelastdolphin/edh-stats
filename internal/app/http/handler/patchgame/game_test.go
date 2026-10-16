package patchgame

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/thelastdolphin/edh-stats/internal/app/model"
	"github.com/thelastdolphin/edh-stats/internal/app/storage"
)

func newTestStore(t *testing.T) *storage.Store {
	t.Helper()

	store, err := storage.NewStore(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("storage.NewStore: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return store
}

func newTestMux(store *storage.Store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /games/{id}", Game(store))
	mux.HandleFunc("PATCH /games/{id}/decks/{deckID}", Participant(store))
	return mux
}

func mustCreateDeck(t *testing.T, store *storage.Store, name, owner string) *model.Deck {
	t.Helper()

	deck, err := store.Deck().Create(context.Background(), &model.Deck{Name: name, Owner: owner})
	if err != nil {
		t.Fatalf("Create deck %q: %v", name, err)
	}
	return deck
}

// seed records one game with a win for the first deck, the way the API would.
func seed(t *testing.T, store *storage.Store) (*model.Game, *model.Deck, *model.Deck) {
	t.Helper()

	ctx := context.Background()
	mine := mustCreateDeck(t, store, "My Deck", "Andrew Kornilov")
	opponent := mustCreateDeck(t, store, "Opponent", "Ivan")

	ids, err := store.Game().ResultTypeIDs(ctx)
	if err != nil {
		t.Fatalf("ResultTypeIDs: %v", err)
	}
	win := ids["win"]

	game, err := store.Game().Create(ctx, &model.Game{
		PlayedAt: "2026-02-07",
		Notes:    "opps conceded",
		Decks: []*model.GameDeck{
			{DeckID: mine.ID, Player: "Andrew Kornilov", ResultTypeID: &win},
			{DeckID: opponent.ID},
		},
	})
	if err != nil {
		t.Fatalf("Create game: %v", err)
	}
	return game, mine, opponent
}

func patch(t *testing.T, mux *http.ServeMux, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequest(http.MethodPatch, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func decodeGame(t *testing.T, recorder *httptest.ResponseRecorder) model.Game {
	t.Helper()

	var game model.Game
	if err := json.Unmarshal(recorder.Body.Bytes(), &game); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body, err)
	}
	return game
}

func decodeCode(t *testing.T, recorder *httptest.ResponseRecorder) string {
	t.Helper()

	var payload struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode error body %q: %v", recorder.Body, err)
	}
	return payload.Error.Code
}

// TestGame_ChangesOnlyWhatItIsGiven is the point of PATCH: correcting the date must
// not touch the notes, and the other way round.
func TestGame_ChangesOnlyWhatItIsGiven(t *testing.T) {
	store := newTestStore(t)
	game, _, _ := seed(t, store)
	mux := newTestMux(store)

	recorder := patch(t, mux, fmt.Sprintf("/games/%d", game.ID), `{"played_at":"2026-05-23"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	got := decodeGame(t, recorder)
	if got.PlayedAt != "2026-05-23" || got.Notes != "opps conceded" {
		t.Errorf("game = %+v, want the new date and the old notes", got)
	}
	if len(got.Decks) != 2 {
		t.Errorf("participants = %d, want the two that were there", len(got.Decks))
	}

	recorder = patch(t, mux, fmt.Sprintf("/games/%d", game.ID), `{"notes":"combat damage"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	got = decodeGame(t, recorder)
	if got.PlayedAt != "2026-05-23" || got.Notes != "combat damage" {
		t.Errorf("game = %+v", got)
	}

	stored, err := store.Game().GetByID(context.Background(), game.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Notes != "combat damage" || stored.PlayedAt != "2026-05-23" {
		t.Errorf("stored game = %+v", stored)
	}
}

func TestGame_RejectsBadPayloads(t *testing.T) {
	store := newTestStore(t)
	game, _, _ := seed(t, store)
	mux := newTestMux(store)

	for name, tc := range map[string]struct {
		body string
		code string
	}{
		"nothing to update":  {`{}`, "validation_failed"},
		"an impossible date": {`{"played_at":"2026-02-30"}`, "validation_failed"},
		"an unknown field":   {`{"date":"2026-05-23"}`, "invalid_json"},
		"broken json":        {`not json`, "invalid_json"},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := patch(t, mux, fmt.Sprintf("/games/%d", game.ID), tc.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusBadRequest, recorder.Body)
			}
			if got := decodeCode(t, recorder); got != tc.code {
				t.Errorf("code = %q, want %q", got, tc.code)
			}
		})
	}
}

func TestGame_NotFound(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	recorder := patch(t, mux, "/games/999999", `{"notes":"x"}`)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNotFound, recorder.Body)
	}
	if got := decodeCode(t, recorder); got != "game_not_found" {
		t.Errorf("code = %q, want game_not_found", got)
	}
}

func TestGame_BadID(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	for _, target := range []string{"/games/abc", "/games/0", "/games/-5"} {
		recorder := patch(t, mux, target, `{"notes":"x"}`)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("PATCH %s = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestParticipant_SetsTheResultAndThePlayer(t *testing.T) {
	store := newTestStore(t)
	game, _, opponent := seed(t, store)
	mux := newTestMux(store)

	recorder := patch(t, mux, fmt.Sprintf("/games/%d/decks/%d", game.ID, opponent.ID),
		`{"player":"Ivan","result":"loss"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	got := decodeGame(t, recorder)
	if len(got.Decks) != 2 {
		t.Fatalf("participants = %d, want 2", len(got.Decks))
	}
	// The whole game comes back, so a client does not have to guess the new state.
	if got.Decks[1].Player != "Ivan" || got.Decks[1].ResultTypeID == nil {
		t.Errorf("changed participation = %+v", got.Decks[1])
	}
	if got.Decks[0].ResultTypeID == nil {
		t.Error("my own result was lost")
	}
}

// TestParticipant_KeepsWhatWasNotSent covers the partial update: fixing the player
// must leave the result alone.
func TestParticipant_KeepsWhatWasNotSent(t *testing.T) {
	store := newTestStore(t)
	game, _, opponent := seed(t, store)
	mux := newTestMux(store)

	if recorder := patch(t, mux, fmt.Sprintf("/games/%d/decks/%d", game.ID, opponent.ID),
		`{"result":"draw"}`); recorder.Code != http.StatusOK {
		t.Fatalf("setting the result: %d (%s)", recorder.Code, recorder.Body)
	}

	recorder := patch(t, mux, fmt.Sprintf("/games/%d/decks/%d", game.ID, opponent.ID), `{"player":"Ivan"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	got := decodeGame(t, recorder)
	if got.Decks[1].Player != "Ivan" {
		t.Errorf("player = %q, want Ivan", got.Decks[1].Player)
	}
	if got.Decks[1].ResultTypeID == nil {
		t.Error("the result set earlier was lost by a patch that did not mention it")
	}
}

// TestParticipant_ClearsTheResult is the one place where an empty string is a value
// rather than a validation error.
func TestParticipant_ClearsTheResult(t *testing.T) {
	store := newTestStore(t)
	game, mine, _ := seed(t, store)
	mux := newTestMux(store)

	recorder := patch(t, mux, fmt.Sprintf("/games/%d/decks/%d", game.ID, mine.ID), `{"result":""}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	if got := decodeGame(t, recorder); got.Decks[0].ResultTypeID != nil {
		t.Errorf("the result was not cleared: %+v", got.Decks[0])
	}
}

func TestParticipant_UnknownDeckInTheGame(t *testing.T) {
	store := newTestStore(t)
	game, _, _ := seed(t, store)
	stranger := mustCreateDeck(t, store, "Stranger", "")
	mux := newTestMux(store)

	recorder := patch(t, mux, fmt.Sprintf("/games/%d/decks/%d", game.ID, stranger.ID), `{"player":"Ivan"}`)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNotFound, recorder.Body)
	}
	if got := decodeCode(t, recorder); got != "participant_not_found" {
		t.Errorf("code = %q, want participant_not_found", got)
	}
}

func TestParticipant_UnknownGame(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "Deck", "Andrew Kornilov")
	mux := newTestMux(store)

	recorder := patch(t, mux, fmt.Sprintf("/games/999999/decks/%d", deck.ID), `{"player":"Ivan"}`)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNotFound, recorder.Body)
	}
	if got := decodeCode(t, recorder); got != "game_not_found" {
		t.Errorf("code = %q, want game_not_found", got)
	}
}

func TestParticipant_RejectsBadInput(t *testing.T) {
	store := newTestStore(t)
	game, _, opponent := seed(t, store)
	mux := newTestMux(store)

	for name, tc := range map[string]struct {
		target string
		body   string
		code   string
	}{
		"nothing to update":       {fmt.Sprintf("/games/%d/decks/%d", game.ID, opponent.ID), `{}`, "validation_failed"},
		"unknown result":          {fmt.Sprintf("/games/%d/decks/%d", game.ID, opponent.ID), `{"result":"victory"}`, "validation_failed"},
		"deck id is not a number": {fmt.Sprintf("/games/%d/decks/abc", game.ID), `{"player":"Ivan"}`, "invalid_id"},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := patch(t, mux, tc.target, tc.body)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusBadRequest, recorder.Body)
			}
			if got := decodeCode(t, recorder); got != tc.code {
				t.Errorf("code = %q, want %q", got, tc.code)
			}
		})
	}
}
