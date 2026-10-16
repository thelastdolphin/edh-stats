package patchdeck

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// newTestMux wires the same pattern the application registers, so that
// r.PathValue("id") is populated exactly as it is in production.
func newTestMux(store *storage.Store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("PATCH /decks/{id}", Deck(store))
	return mux
}

func mustCreateDeck(t *testing.T, store *storage.Store, name, owner, deckType string) *model.Deck {
	t.Helper()

	deck, err := store.Deck().Create(context.Background(), &model.Deck{
		Name:  name,
		Owner: owner,
		Type:  deckType,
	})
	if err != nil {
		t.Fatalf("Create(%q): %v", name, err)
	}
	return deck
}

func doJSON(t *testing.T, mux *http.ServeMux, target, body string) *httptest.ResponseRecorder {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request := httptest.NewRequest(http.MethodPatch, target, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)
	return recorder
}

func decodeDeck(t *testing.T, recorder *httptest.ResponseRecorder) model.Deck {
	t.Helper()

	var deck model.Deck
	if err := json.Unmarshal(recorder.Body.Bytes(), &deck); err != nil {
		t.Fatalf("decode response %q: %v", recorder.Body, err)
	}
	return deck
}

// TestDeck_KeepsTheFieldsThatWereNotSent is the essence of PATCH: renaming a deck
// must not touch its owner or its type.
func TestDeck_KeepsTheFieldsThatWereNotSent(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "Clement Frogs", "Andrew Kornilov", "control")
	mux := newTestMux(store)

	recorder := doJSON(t, mux, fmt.Sprintf("/decks/%d", deck.ID), `{"name":"Clement Bounce"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	got := decodeDeck(t, recorder)
	if got.Name != "Clement Bounce" {
		t.Errorf("name = %q, want %q", got.Name, "Clement Bounce")
	}
	if got.Owner != "Andrew Kornilov" || got.Type != "control" {
		t.Errorf("the fields that were not sent changed: %+v", got)
	}

	stored, err := store.Deck().GetByID(context.Background(), deck.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Name != "Clement Bounce" || stored.Owner != "Andrew Kornilov" || stored.Type != "control" {
		t.Errorf("stored deck = %+v", stored)
	}
}

func TestDeck_UpdatesEveryFieldItIsGiven(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "Before", "someone", "control")
	mux := newTestMux(store)

	recorder := doJSON(t, mux, fmt.Sprintf("/decks/%d", deck.ID),
		`{"name":"After","owner":"Andrew Kornilov","type":"combo"}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	got := decodeDeck(t, recorder)
	if got != (model.Deck{ID: deck.ID, Name: "After", Owner: "Andrew Kornilov", Type: "combo"}) {
		t.Errorf("deck = %+v", got)
	}
}

func TestDeck_TrimsTheValuesItStores(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "Deck", "Andrew Kornilov", "control")
	mux := newTestMux(store)

	recorder := doJSON(t, mux, fmt.Sprintf("/decks/%d", deck.ID), `{"name":"  Deck  "}`)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}
	if got := decodeDeck(t, recorder); got.Name != "Deck" {
		t.Errorf("name = %q, want %q", got.Name, "Deck")
	}
}

func TestDeck_NotFound(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	recorder := doJSON(t, mux, "/decks/999999", `{"name":"x"}`)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNotFound, recorder.Body)
	}
}

func TestDeck_BadID(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	for _, target := range []string{"/decks/abc", "/decks/0", "/decks/-5"} {
		recorder := doJSON(t, mux, target, `{"name":"x"}`)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("PATCH %s = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}

func TestDeck_DuplicateNameAndOwner(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()
	mustCreateDeck(t, store, "Taken", "Andrew Kornilov", "control")
	deck := mustCreateDeck(t, store, "Free", "Andrew Kornilov", "control")
	mux := newTestMux(store)

	recorder := doJSON(t, mux, fmt.Sprintf("/decks/%d", deck.ID), `{"name":"Taken"}`)
	if recorder.Code != http.StatusConflict {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusConflict, recorder.Body)
	}

	// The deck keeps its name: a refused update changes nothing.
	stored, err := store.Deck().GetByID(ctx, deck.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if stored.Name != "Free" {
		t.Errorf("name = %q, want %q", stored.Name, "Free")
	}
}

func TestDeck_InvalidBody(t *testing.T) {
	store := newTestStore(t)
	deck := mustCreateDeck(t, store, "Deck", "Andrew Kornilov", "control")
	mux := newTestMux(store)
	longName := strings.Repeat("a", 51)

	for name, body := range map[string]string{
		"nothing to update": `{}`,
		"unknown field":     `{"nickname":"x"}`,
		"empty name":        `{"name":""}`,
		"blank name":        `{"name":"   "}`,
		"name too long":     fmt.Sprintf(`{"name":%q}`, longName),
		"empty owner":       `{"owner":""}`,
		"not json":          `not json`,
		"two documents":     `{"name":"a"}{"name":"b"}`,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := doJSON(t, mux, fmt.Sprintf("/decks/%d", deck.ID), body)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusBadRequest, recorder.Body)
			}
		})
	}
}
