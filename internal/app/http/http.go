package http

import (
	"encoding/json"
	"log"
	"net/http"
)

func JSONEncode(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("error encoding json: %v", err)
	}
}

func Error(w http.ResponseWriter, status int, message string) {
	JSONEncode(w, status, map[string]string{"error": message})
}
