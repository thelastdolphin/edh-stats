package create

import (
	"context"
	"encoding/json"
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

func postDeck(t *testing.T, store *storage.Store, body string) *httptest.ResponseRecorder {
	t.Helper()

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/decks", strings.NewReader(body))
	Deck(store)(recorder, request)
	return recorder
}

func TestDeck_Created(t *testing.T) {
	store := newTestStore(t)

	recorder := postDeck(t, store, `{"name":"My Deck","owner":"tester","type":"control"}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusCreated, recorder.Body)
	}
	if location := recorder.Header().Get("Location"); location != "/decks/1" {
		t.Errorf("Location = %q, want /decks/1", location)
	}

	var deck model.Deck
	if err := json.Unmarshal(recorder.Body.Bytes(), &deck); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if deck.ID != 1 || deck.Name != "My Deck" || deck.Owner != "tester" || deck.Type != "control" {
		t.Errorf("unexpected deck: %+v", deck)
	}
	// The response must use the same field names the API accepts on input.
	if !strings.Contains(recorder.Body.String(), `"name"`) {
		t.Errorf("response does not use lowercase json field names: %s", recorder.Body)
	}
}

func TestDeck_TrimsWhitespace(t *testing.T) {
	store := newTestStore(t)

	recorder := postDeck(t, store, `{"name":"  My Deck  ","owner":" tester ","type":" control "}`)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d (body: %s)", recorder.Code, http.StatusCreated, recorder.Body)
	}

	var deck model.Deck
	if err := json.Unmarshal(recorder.Body.Bytes(), &deck); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if deck.Name != "My Deck" || deck.Owner != "tester" || deck.Type != "control" {
		t.Errorf("values were not trimmed: %+v", deck)
	}

	// Because of trimming, " My Deck " must collide with the stored "My Deck".
	duplicate := postDeck(t, store, `{"name":"My Deck","owner":"tester","type":"control"}`)
	if duplicate.Code != http.StatusConflict {
		t.Errorf("duplicate status = %d, want %d", duplicate.Code, http.StatusConflict)
	}
}

// TestDeck_DuplicateConflict is the regression test for the previous
// strings.Contains(err.Error(), "already exists") check, which never matched
// SQLite's "UNIQUE constraint failed: ..." message and answered 500.
func TestDeck_DuplicateConflict(t *testing.T) {
	store := newTestStore(t)
	const body = `{"name":"Test Deck","owner":"tester","type":"control"}`

	if recorder := postDeck(t, store, body); recorder.Code != http.StatusCreated {
		t.Fatalf("first create = %d, want %d (body: %s)", recorder.Code, http.StatusCreated, recorder.Body)
	}

	recorder := postDeck(t, store, body)
	if recorder.Code != http.StatusConflict {
		t.Errorf("second create = %d, want %d (body: %s)", recorder.Code, http.StatusConflict, recorder.Body)
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("error Content-Type = %q, want application/json", contentType)
	}
}

func TestDeck_ValidationErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing name", body: `{"owner":"tester","type":"control"}`},
		{name: "blank name", body: `{"name":"   ","owner":"tester","type":"control"}`},
		{name: "missing owner", body: `{"name":"deck","type":"control"}`},
		{name: "missing type", body: `{"name":"deck","owner":"tester"}`},
		{name: "name too long", body: `{"name":"` + strings.Repeat("a", 51) + `","owner":"tester","type":"control"}`},
		{name: "owner too long", body: `{"name":"deck","owner":"` + strings.Repeat("a", 51) + `","type":"control"}`},
		{name: "type too long", body: `{"name":"deck","owner":"tester","type":"` + strings.Repeat("a", 51) + `"}`},
		{name: "unknown field", body: `{"name":"deck","owner":"tester","type":"control","extra":1}`},
		{name: "malformed json", body: `{"name":`},
		{name: "two json objects", body: `{"name":"deck","owner":"tester","type":"control"}{"name":"other"}`},
	}

	store := newTestStore(t)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			recorder := postDeck(t, store, test.body)
			if recorder.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusBadRequest, recorder.Body)
			}
			if !strings.Contains(recorder.Body.String(), `"error"`) {
				t.Errorf("error body is not a JSON envelope: %s", recorder.Body)
			}
		})
	}
}

// TestDeck_TypeIsFreeText pins the agreed contract: type is free text limited
// to 50 characters, not a fixed list of archetypes.
func TestDeck_TypeIsFreeText(t *testing.T) {
	store := newTestStore(t)

	recorder := postDeck(t, store, `{"name":"deck","owner":"tester","type":"totally-bogus"}`)
	if recorder.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusCreated, recorder.Body)
	}
}

// TestDeck_BodyTooLarge covers the unbounded request body: a 20 MB "name" used
// to be accepted and stored.
func TestDeck_BodyTooLarge(t *testing.T) {
	store := newTestStore(t)

	huge := `{"name":"` + strings.Repeat("a", maxBodyBytes) + `","owner":"tester","type":"control"}`
	recorder := postDeck(t, store, huge)

	if recorder.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want %d (body: %s)", recorder.Code, http.StatusRequestEntityTooLarge, recorder.Body)
	}
}
