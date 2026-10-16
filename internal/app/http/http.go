// Package http contains the shared HTTP helpers used by the handlers.
package http

import (
	"encoding/json"
	"log"
	"net/http"
)

// JSONEncode writes data as a JSON response body.
//
// The payload is marshalled before the status line is written: encoding
// straight into the ResponseWriter sends the status first and would leave the
// client with a truncated body and a success status if encoding then failed.
func JSONEncode(w http.ResponseWriter, status int, data any) {
	body, err := json.Marshal(data)
	if err != nil {
		log.Printf("encode json response: %v", err)
		writeRaw(w, http.StatusInternalServerError, encodeError(CodeInternal, "internal error"))
		return
	}
	writeRaw(w, status, string(body))
}

// Error writes a JSON error response.
//
// The envelope is an object rather than a bare string so that a client can act on
// the code without reading the message, and so that more fields - the offending
// field of a validation failure, for instance - can be added later without
// breaking anyone.
func Error(w http.ResponseWriter, status int, code, message string) {
	writeRaw(w, status, encodeError(code, message))
}

type errorEnvelope struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// encodeError marshals the envelope, with a hand-written body as the last resort:
// this runs on an error path already, and a second failure must still produce
// valid JSON for the client.
func encodeError(code, message string) string {
	body, err := json.Marshal(errorEnvelope{Error: errorDetail{Code: code, Message: message}})
	if err != nil {
		return `{"error":{"code":"internal_error","message":"internal error"}}`
	}
	return string(body)
}

func writeRaw(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body + "\n")); err != nil {
		log.Printf("write response body: %v", err)
	}
}
