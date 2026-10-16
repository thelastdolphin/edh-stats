package getdeck

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

// newTestMux wires the same patterns the application registers, so that
// r.PathValue("id") is populated exactly as it is in production.
func newTestMux(store *storage.Store) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /decks", ListDecks(store))
	mux.HandleFunc("GET /decks/{id}", GetDeck(store))
	return mux
}

func mustCreateDeck(t *testing.T, store *storage.Store, name string) *model.Deck {
	t.Helper()

	deck, err := store.Deck().Create(context.Background(), &model.Deck{
		Name:  name,
		Owner: "tester",
		Type:  "control",
	})
	if err != nil {
		t.Fatalf("Create(%q): %v", name, err)
	}
	return deck
}

func get(t *testing.T, mux *http.ServeMux, target string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func TestGetDeck_Found(t *testing.T) {
	store := newTestStore(t)
	created := mustCreateDeck(t, store, "My Deck")
	mux := newTestMux(store)

	recorder := get(t, mux, fmt.Sprintf("/decks/%d", created.ID))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	var deck model.Deck
	if err := json.Unmarshal(recorder.Body.Bytes(), &deck); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if deck.ID != created.ID || deck.Name != "My Deck" {
		t.Errorf("unexpected deck: %+v", deck)
	}
}

// TestGetDeck_NotFoundIs404 is the regression test for the lost sql.ErrNoRows:
// the repository returned a fresh error without wrapping it, so errors.Is never
// matched and a missing deck answered 500.
func TestGetDeck_NotFoundIs404(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	recorder := get(t, mux, "/decks/999999")
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusNotFound, recorder.Body)
	}
}

func TestGetDeck_BadID(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	for _, target := range []string{"/decks/abc", "/decks/-5", "/decks/0", "/decks/1.5"} {
		recorder := get(t, mux, target)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}

// TestListDecks_EmptyIsJSONArray guards the response shape: an empty page must
// be [] rather than null.
func TestListDecks_EmptyIsJSONArray(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	recorder := get(t, mux, "/decks")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if body := strings.TrimSpace(recorder.Body.String()); body != "[]" {
		t.Errorf("body = %s, want []", body)
	}
}

// TestListDecks_Pagination covers the page/limit parameters that used to be
// parsed and then ignored by the repository.
func TestListDecks_Pagination(t *testing.T) {
	store := newTestStore(t)
	for i := 1; i <= 5; i++ {
		mustCreateDeck(t, store, fmt.Sprintf("deck-%d", i))
	}
	mux := newTestMux(store)

	recorder := get(t, mux, "/decks?page=2&limit=2")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
	}

	var decks []model.Deck
	if err := json.Unmarshal(recorder.Body.Bytes(), &decks); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(decks) != 2 || decks[0].Name != "deck-3" || decks[1].Name != "deck-4" {
		t.Errorf("page 2 = %+v, want deck-3 and deck-4", decks)
	}

	// limit is capped, page defaults to 1
	defaulted := get(t, mux, "/decks?limit=2")
	if err := json.Unmarshal(defaulted.Body.Bytes(), &decks); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(decks) != 2 || decks[0].Name != "deck-1" {
		t.Errorf("default page = %+v, want the first two decks", decks)
	}
}

func TestListDecks_InvalidPagination(t *testing.T) {
	store := newTestStore(t)
	mux := newTestMux(store)

	for _, target := range []string{
		"/decks?limit=0",
		"/decks?limit=101",
		"/decks?limit=abc",
		"/decks?page=0",
		"/decks?page=-2",
		"/decks?page=abc",
	} {
		recorder := get(t, mux, target)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("GET %s = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}

// TestListDecks_Search covers the substring search the deck picker of a client is
// built on.
func TestListDecks_Search(t *testing.T) {
	store := newTestStore(t)
	mustCreateDeck(t, store, "Clement Frogs")
	mustCreateDeck(t, store, "Wilhelt Zombies")
	mux := newTestMux(store)

	for name, tc := range map[string]struct {
		target string
		want   int
	}{
		"part of a name":       {"/decks?search=frog", 1},
		"part of an owner":     {"/decks?search=test", 2},
		"case does not matter": {"/decks?search=WILHELT", 1},
		"nothing matches":      {"/decks?search=zzz", 0},
		"the whole list":       {"/decks", 2},
		"a wildcard is text":   {"/decks?search=_", 0},
	} {
		t.Run(name, func(t *testing.T) {
			recorder := get(t, mux, tc.target)
			if recorder.Code != http.StatusOK {
				t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusOK, recorder.Body)
			}

			var decks []model.Deck
			if err := json.Unmarshal(recorder.Body.Bytes(), &decks); err != nil {
				t.Fatalf("decode %q: %v", recorder.Body, err)
			}
			if len(decks) != tc.want {
				t.Errorf("decks = %d, want %d", len(decks), tc.want)
			}
		})
	}
}
