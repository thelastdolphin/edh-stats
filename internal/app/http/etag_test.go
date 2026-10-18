package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// listing is a handler shaped like the real ones: a header, a status and a body.
func listing(body string, total string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Total-Count", total)
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(body)); err != nil {
			return
		}
	})
}

func TestETagTagsAnAnswer(t *testing.T) {
	handler := ETag(listing(`[{"id":1}]`, "1"))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/decks", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if tag := recorder.Header().Get(etagHeader); tag == "" {
		t.Error("no ETag on the answer")
	}
	if body := recorder.Body.String(); body != `[{"id":1}]` {
		t.Errorf("body = %q, want the handler's", body)
	}
	if total := recorder.Header().Get("X-Total-Count"); total != "1" {
		t.Errorf("X-Total-Count = %q, want the handler's", total)
	}
}

func TestETagAnswersARepeatWithoutABody(t *testing.T) {
	handler := ETag(listing(`[{"id":1}]`, "1"))

	first := httptest.NewRecorder()
	handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/decks", nil))
	tag := first.Header().Get(etagHeader)

	second := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/decks", nil)
	request.Header.Set(ifNoneMatchHeader, tag)
	handler.ServeHTTP(second, request)

	if second.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", second.Code, http.StatusNotModified)
	}
	if second.Body.Len() != 0 {
		t.Errorf("a 304 carried a body: %q", second.Body.String())
	}
	if second.Header().Get(etagHeader) != tag {
		t.Error("a 304 must repeat the tag it is about")
	}
	if second.Header().Get("X-Total-Count") != "1" {
		t.Error("a 304 must keep the headers of the answer")
	}
}

func TestETagAnswersAStaleTagWithTheBody(t *testing.T) {
	handler := ETag(listing(`[{"id":1}]`, "1"))

	request := httptest.NewRequest(http.MethodGet, "/decks", nil)
	request.Header.Set(ifNoneMatchHeader, `"stale"`)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	if recorder.Body.Len() == 0 {
		t.Error("a stale tag must be answered with the body")
	}
}

func TestETagAcceptsAListOfTags(t *testing.T) {
	handler := ETag(listing(`[]`, "0"))

	tag := tagOf([]byte(`[]`))

	request := httptest.NewRequest(http.MethodGet, "/decks", nil)
	request.Header.Set(ifNoneMatchHeader, `"other", `+tag)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotModified)
	}
}

func TestETagAcceptsTheWildcard(t *testing.T) {
	handler := ETag(listing(`[]`, "0"))

	request := httptest.NewRequest(http.MethodGet, "/decks", nil)
	request.Header.Set(ifNoneMatchHeader, "*")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotModified)
	}
}

func TestETagLeavesOtherMethodsAlone(t *testing.T) {
	called := false
	handler := ETag(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusCreated)
		if _, err := w.Write([]byte(`{"id":2}`)); err != nil {
			return
		}
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/games", nil))

	if !called {
		t.Fatal("the handler was not reached")
	}
	if recorder.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusCreated)
	}
	if recorder.Header().Get(etagHeader) != "" {
		t.Error("a creation must not be tagged")
	}
	if recorder.Body.String() != `{"id":2}` {
		t.Errorf("body = %q, want the handler's", recorder.Body.String())
	}
}

func TestETagPassesARefusalThrough(t *testing.T) {
	handler := ETag(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		if _, err := w.Write([]byte(`{"error":{"code":"deck_not_found"}}`)); err != nil {
			return
		}
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/decks/9", nil))

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if recorder.Header().Get(etagHeader) != "" {
		t.Error("a refusal must not be tagged")
	}
	if recorder.Body.Len() == 0 {
		t.Error("the refusal lost its body")
	}
}

func TestTagOfChangesWithTheBody(t *testing.T) {
	if tagOf([]byte("a")) == tagOf([]byte("b")) {
		t.Error("two different bodies got the same tag")
	}

	// Two calls, two variables: comparing the expression with itself would be a check
	// that can never fail, and a linter is right to say so.
	first := tagOf([]byte("a"))
	second := tagOf([]byte("a"))
	if first != second {
		t.Error("the same body got two tags")
	}
	if got := tagOf([]byte("a")); got == "" || got[0] != '"' || got[len(got)-1] != '"' {
		t.Errorf("tag %q is not quoted", got)
	}
}
