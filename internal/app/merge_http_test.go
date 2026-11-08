package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// doJSON sends a request through the real handler chain and decodes the answer.
//
// The endpoints under test change data, so going through the router rather than calling the
// repository is the point: the status codes, the JSON bodies and the error envelopes are
// what a client actually sees.
func doJSON(t *testing.T, application *Application, method, target, body string, out any) int {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	application.Server.Handler.ServeHTTP(recorder, request)

	if out != nil && recorder.Body.Len() > 0 {
		if err := json.Unmarshal(recorder.Body.Bytes(), out); err != nil {
			t.Fatalf("%s %s: decode %q: %v", method, target, recorder.Body.String(), err)
		}
	}
	return recorder.Code
}

type idOnly struct {
	ID int64 `json:"id"`
}

func mustCreateDeck(t *testing.T, application *Application, name, owner string) int64 {
	t.Helper()

	var deck idOnly
	body := fmt.Sprintf(`{"name":%q,"owner":%q,"type":"commander"}`, name, owner)
	if code := doJSON(t, application, http.MethodPost, "/decks", body, &deck); code != http.StatusCreated {
		t.Fatalf("create deck %q: status %d", name, code)
	}
	return deck.ID
}

func mustCreateGame(t *testing.T, application *Application, participants ...string) int64 {
	t.Helper()

	var game idOnly
	body := fmt.Sprintf(`{"played_at":"2026-02-21","notes":"","decks":[%s]}`, strings.Join(participants, ","))
	if code := doJSON(t, application, http.MethodPost, "/games", body, &game); code != http.StatusCreated {
		t.Fatalf("create game: status %d", code)
	}
	return game.ID
}

type gameDecks struct {
	Decks []struct {
		DeckID int64  `json:"deck_id"`
		Player string `json:"player"`
	} `json:"decks"`
}

func TestMergeDeckMovesTheGamesAndDeletesTheSource(t *testing.T) {
	application := newTestApp(t)

	source := mustCreateDeck(t, application, "Blim Gifts", "Andrew Kornilov")
	target := mustCreateDeck(t, application, "Blim Gifts", "Georgy Borodin")
	opponent := mustCreateDeck(t, application, "Arjun Draw", "Ivan")
	game := mustCreateGame(t, application,
		fmt.Sprintf(`{"deck_id":%d,"player":"Andrew","result":"win"}`, source),
		fmt.Sprintf(`{"deck_id":%d,"player":"Ivan","result":""}`, opponent),
	)

	var merged struct {
		Moved int `json:"moved"`
	}
	code := doJSON(t, application, http.MethodPost, fmt.Sprintf("/decks/%d/merge", source),
		fmt.Sprintf(`{"into":%d}`, target), &merged)
	if code != http.StatusOK {
		t.Fatalf("merge: status %d, want 200", code)
	}
	if merged.Moved != 1 {
		t.Fatalf("moved = %d, want 1", merged.Moved)
	}

	if code := doJSON(t, application, http.MethodGet, fmt.Sprintf("/decks/%d", source), "", nil); code != http.StatusNotFound {
		t.Fatalf("the merged deck still answers: %d", code)
	}

	var after gameDecks
	doJSON(t, application, http.MethodGet, fmt.Sprintf("/games/%d", game), "", &after)
	assertParticipant(t, after, target, "Andrew")
	if hasDeck(after, source) {
		t.Fatalf("the game still points at the merged-away deck: %+v", after.Decks)
	}
}

// hasDeck reports whether the game still holds a participation of that deck.
func hasDeck(game gameDecks, deckID int64) bool {
	for _, participant := range game.Decks {
		if participant.DeckID == deckID {
			return true
		}
	}
	return false
}

// assertParticipant checks that the game holds exactly one participation of the deck by
// that player - the point of a move or a merge is which deck a row points at, not how many
// rows the game has.
func assertParticipant(t *testing.T, game gameDecks, deckID int64, player string) {
	t.Helper()

	for _, participant := range game.Decks {
		if participant.DeckID == deckID && participant.Player == player {
			return
		}
	}
	t.Fatalf("no participation of deck %d by %q in %+v", deckID, player, game.Decks)
}

func TestMergeDeckRefusals(t *testing.T) {
	application := newTestApp(t)

	deck := mustCreateDeck(t, application, "Blim Gifts", "Georgy Borodin")

	tests := []struct {
		name   string
		target string
		body   string
		want   int
	}{
		{
			name:   "into itself",
			target: fmt.Sprintf("/decks/%d/merge", deck),
			body:   fmt.Sprintf(`{"into":%d}`, deck),
			want:   http.StatusBadRequest,
		},
		{
			name:   "into a deck that does not exist",
			target: fmt.Sprintf("/decks/%d/merge", deck),
			body:   fmt.Sprintf(`{"into":%d}`, deck+99),
			want:   http.StatusNotFound,
		},
		{
			name:   "without saying into which deck",
			target: fmt.Sprintf("/decks/%d/merge", deck),
			body:   `{}`,
			want:   http.StatusBadRequest,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if code := doJSON(t, application, http.MethodPost, test.target, test.body, nil); code != test.want {
				t.Fatalf("status %d, want %d", code, test.want)
			}
		})
	}

	// the deck survived every refusal
	if code := doJSON(t, application, http.MethodGet, fmt.Sprintf("/decks/%d", deck), "", nil); code != http.StatusOK {
		t.Fatalf("the deck was harmed by a refused merge: %d", code)
	}
}

func TestMoveParticipantToAnotherDeck(t *testing.T) {
	application := newTestApp(t)

	wrong := mustCreateDeck(t, application, "Wilhelt Zombies (AA)", "Alexander Agassiz")
	right := mustCreateDeck(t, application, "Wilhelt Zombies", "Georgy Borodin")
	opponent := mustCreateDeck(t, application, "Arjun Draw", "Andrew Kornilov")
	game := mustCreateGame(t, application,
		fmt.Sprintf(`{"deck_id":%d,"player":"Georgy","result":"win"}`, wrong),
		fmt.Sprintf(`{"deck_id":%d,"player":"Andrew","result":""}`, opponent),
	)

	path := fmt.Sprintf("/games/%d/decks/%d", game, wrong)
	if code := doJSON(t, application, http.MethodPatch, path, fmt.Sprintf(`{"deck_id":%d}`, right), nil); code != http.StatusOK {
		t.Fatalf("move: status %d, want 200", code)
	}

	var after gameDecks
	doJSON(t, application, http.MethodGet, fmt.Sprintf("/games/%d", game), "", &after)
	assertParticipant(t, after, right, "Georgy")
	if hasDeck(after, wrong) {
		t.Fatalf("the participation still points at the old deck: %+v", after.Decks)
	}
}

func TestMoveParticipantOntoADeckAlreadyInTheGameIsRefused(t *testing.T) {
	application := newTestApp(t)

	first := mustCreateDeck(t, application, "Liesa Lifegain/Lifeloss", "Andrew Kornilov")
	second := mustCreateDeck(t, application, "Arjun Draw", "Andrew Kornilov")
	game := mustCreateGame(t, application,
		fmt.Sprintf(`{"deck_id":%d,"player":"Andrew","result":"win"}`, first),
		fmt.Sprintf(`{"deck_id":%d,"player":"Ivan","result":""}`, second),
	)

	path := fmt.Sprintf("/games/%d/decks/%d", game, first)
	code := doJSON(t, application, http.MethodPatch, path, fmt.Sprintf(`{"deck_id":%d}`, second), nil)
	if code != http.StatusConflict {
		t.Fatalf("status %d, want 409", code)
	}
}
