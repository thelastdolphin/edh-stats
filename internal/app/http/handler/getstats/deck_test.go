package getstats

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
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
	mux.HandleFunc("GET /stats/decks", DeckStats(store))
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

func resultID(t *testing.T, store *storage.Store, name string) *int64 {
	t.Helper()

	ids, err := store.Game().ResultTypeIDs(context.Background())
	if err != nil {
		t.Fatalf("ResultTypeIDs: %v", err)
	}
	id, ok := ids[name]
	if !ok {
		t.Fatalf("result type %q is missing", name)
	}
	return &id
}

// seedGames records three games: one win and one loss for my deck, and a third game
// with no result at all, so that "games" and "recorded results" can be told apart.
func seedGames(t *testing.T, store *storage.Store) {
	t.Helper()

	ctx := context.Background()
	mine := mustCreateDeck(t, store, "My Deck", "Andrew Kornilov")
	opponent := mustCreateDeck(t, store, "Opponent", "Ivan")

	games := []struct {
		date   string
		result *int64
	}{
		{"2026-02-07", resultID(t, store, "win")},
		{"2026-05-23", resultID(t, store, "loss")},
		{"2026-10-10", nil},
	}
	for _, game := range games {
		if _, err := store.Game().Create(ctx, &model.Game{
			PlayedAt: game.date,
			Decks: []*model.GameDeck{
				{DeckID: mine.ID, Player: "Andrew Kornilov", ResultTypeID: game.result},
				{DeckID: opponent.ID},
			},
		}); err != nil {
			t.Fatalf("Create game: %v", err)
		}
	}
}

func get(t *testing.T, mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func decodeStats(t *testing.T, recorder *httptest.ResponseRecorder) []model.DeckStats {
	t.Helper()

	var stats []model.DeckStats
	if err := json.Unmarshal(recorder.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body, err)
	}
	return stats
}

func TestDeckStats(t *testing.T) {
	store := newTestStore(t)
	seedGames(t, store)
	mux := newTestMux(store)

	recorder := get(t, mux, "/stats/decks")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	stats := decodeStats(t, recorder)
	if len(stats) != 2 {
		t.Fatalf("rows = %d, want 2 (one per deck that played)", len(stats))
	}

	mine := stats[0]
	if mine.Name != "My Deck" || mine.Owner != "Andrew Kornilov" {
		t.Errorf("first row = %+v", mine)
	}
	if mine.Games != 3 || mine.Wins != 1 || mine.Losses != 1 || mine.Draws != 0 {
		t.Errorf("my statistics = %+v, want 3 games, 1 win, 1 loss", mine)
	}

	// The opponent's deck played the same three games, but no result was recorded
	// for it: zeroes, not invented data.
	opponent := stats[1]
	if opponent.Games != 3 || opponent.Wins != 0 || opponent.Losses != 0 {
		t.Errorf("opponent statistics = %+v, want 3 games and no results", opponent)
	}
}

func TestDeckStats_DateRange(t *testing.T) {
	store := newTestStore(t)
	seedGames(t, store)
	mux := newTestMux(store)

	for name, tc := range map[string]struct {
		target string
		games  int
		wins   int
	}{
		"everything":       {"/stats/decks", 3, 1},
		"from a date":      {"/stats/decks?from=2026-05-23", 2, 0},
		"to a date":        {"/stats/decks?to=2026-02-07", 1, 1},
		"a closed range":   {"/stats/decks?from=2026-02-01&to=2026-06-01", 2, 1},
		"outside the data": {"/stats/decks?from=2027-01-01", 0, 0},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := get(t, mux, tc.target)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
			}

			stats := decodeStats(t, recorder)
			if tc.games == 0 {
				if len(stats) != 0 {
					t.Errorf("rows = %d, want none", len(stats))
				}
				return
			}
			if len(stats) == 0 {
				t.Fatalf("no rows for %s", tc.target)
			}
			if stats[0].Games != tc.games || stats[0].Wins != tc.wins {
				t.Errorf("statistics = %+v, want %d games and %d win(s)", stats[0], tc.games, tc.wins)
			}
		})
	}
}

func TestDeckStats_EmptyIsAnArray(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	recorder := get(t, mux, "/stats/decks")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := recorder.Body.String(); body != "[]\n" {
		t.Errorf("body = %q, want an empty array", body)
	}
}

func TestDeckStats_RejectsBadDates(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	for name, target := range map[string]string{
		"from is not a date": "/stats/decks?from=01.02.26",
		"to is impossible":   "/stats/decks?to=2026-02-30",
	} {
		t.Run(name, func(t *testing.T) {
			recorder := get(t, mux, target)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusBadRequest, recorder.Body)
			}

			var payload struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode error body %q: %v", recorder.Body, err)
			}
			if payload.Error.Code != "invalid_query" {
				t.Errorf("code = %q, want invalid_query", payload.Error.Code)
			}
		})
	}
}
