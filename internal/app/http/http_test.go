package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJSONEncode(t *testing.T) {
	t.Run("writes the payload with the given status", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		JSONEncode(recorder, http.StatusCreated, map[string]string{"status": "ok"})

		if recorder.Code != http.StatusCreated {
			t.Errorf("status = %d, want %d", recorder.Code, http.StatusCreated)
		}
		if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
			t.Errorf("Content-Type = %q", contentType)
		}

		var payload map[string]string
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("body is not valid JSON: %v (%s)", err, recorder.Body)
		}
		if payload["status"] != "ok" {
			t.Errorf("payload = %v, want status=ok", payload)
		}
	})

	// This is why the payload is marshalled before the status line is written:
	// a channel cannot be encoded, and the client must still receive a
	// well-formed JSON error with 500 instead of a truncated success response.
	t.Run("falls back to a JSON error when encoding fails", func(t *testing.T) {
		recorder := httptest.NewRecorder()

		JSONEncode(recorder, http.StatusOK, make(chan int))

		if recorder.Code != http.StatusInternalServerError {
			t.Errorf("status = %d, want %d", recorder.Code, http.StatusInternalServerError)
		}

		var payload map[string]string
		if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
			t.Fatalf("body is not valid JSON: %v (%s)", err, recorder.Body)
		}
		if payload["error"] == "" {
			t.Errorf("payload = %v, want an error field", payload)
		}
	})
}

func TestLogRequests(t *testing.T) {
	recorder := httptest.NewRecorder()
	handler := LogRequests(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/decks", nil))

	if recorder.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusTeapot)
	}
}
