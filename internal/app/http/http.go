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
		writeRaw(w, http.StatusInternalServerError, `{"error":"internal error"}`)
		return
	}
	writeRaw(w, status, string(body))
}

// Error writes a JSON error envelope.
func Error(w http.ResponseWriter, status int, message string) {
	JSONEncode(w, status, map[string]string{"error": message})
}

func writeRaw(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body + "\n")); err != nil {
		log.Printf("write response body: %v", err)
	}
}
