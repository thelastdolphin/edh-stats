package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// maxBodyBytes caps every request body. Without a cap a client can send an
// arbitrarily large payload and exhaust memory and disk.
const maxBodyBytes = 1 << 20 // 1 MiB

// DecodeJSON reads a single JSON object from the request into dst.
//
// It writes the error response itself and reports whether the caller should carry
// on, so that every endpoint answers the same way to a body that is too large, is
// not JSON, or holds two documents instead of one. Unknown fields are refused: a
// misspelled field should fail loudly rather than be dropped silently, which for a
// PATCH would look like success.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			Error(w, http.StatusRequestEntityTooLarge, CodeBodyTooLarge,
				fmt.Sprintf("request body must not exceed %d bytes", maxBodyBytes))
			return false
		}
		Error(w, http.StatusBadRequest, CodeInvalidJSON, "invalid JSON body: "+err.Error())
		return false
	}

	// Anything but EOF here means a second JSON document was sent.
	if err := decoder.Decode(new(struct{})); !errors.Is(err, io.EOF) {
		Error(w, http.StatusBadRequest, CodeInvalidJSON, "body must contain a single JSON object")
		return false
	}
	return true
}
