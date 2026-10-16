package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// decodeError decodes the error envelope written by Error.
func decodeError(t *testing.T, recorder *httptest.ResponseRecorder) errorDetail {
	t.Helper()

	var payload errorEnvelope
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("body is not a valid error envelope: %v (%s)", err, recorder.Body)
	}
	return payload.Error
}

func TestError(t *testing.T) {
	recorder := httptest.NewRecorder()

	Error(recorder, http.StatusNotFound, CodeDeckNotFound, "deck not found")

	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if contentType := recorder.Header().Get("Content-Type"); contentType != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", contentType)
	}

	got := decodeError(t, recorder)
	if got.Code != CodeDeckNotFound {
		t.Errorf("code = %q, want %q", got.Code, CodeDeckNotFound)
	}
	if got.Message != "deck not found" {
		t.Errorf("message = %q", got.Message)
	}
}

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
		if got := decodeError(t, recorder); got.Code != CodeInternal {
			t.Errorf("code = %q, want %q", got.Code, CodeInternal)
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
