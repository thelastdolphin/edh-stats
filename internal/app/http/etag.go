package http

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
)

// etagHeader and ifNoneMatchHeader are named here so that a test and the middleware
// cannot disagree about spelling.
const (
	etagHeader        = "ETag"
	ifNoneMatchHeader = "If-None-Match"
)

// ETag answers a conditional GET with 304 when the client already has the answer.
//
// The listings of this service are small and change rarely, and a phone asks for them
// every time it is opened. With a tag the second answer has no body at all: the client
// keeps what it had. The tag is a hash of the body, which is the whole contract - if a
// handler starts returning something else, the tag changes with it, and there is nothing
// to keep in sync by hand.
//
// Only GET is covered, and only a 200 gets a tag: a refusal or a creation is not something
// a client revalidates.
func ETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			next.ServeHTTP(w, r)
			return
		}

		buffered := &etagRecorder{ResponseWriter: w, header: make(http.Header)}
		next.ServeHTTP(buffered, r)

		// Anything but a complete answer goes out as the handler wrote it.
		if buffered.status != http.StatusOK {
			buffered.flush()
			return
		}

		tag := tagOf(buffered.body)

		// The headers the handler set are sent with both answers: a 304 still has to carry
		// what tells the client what it is a 304 of.
		for name, values := range buffered.header {
			for _, value := range values {
				w.Header().Add(name, value)
			}
		}
		w.Header().Set(etagHeader, tag)

		if matchesIfNoneMatch(r.Header.Get(ifNoneMatchHeader), tag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(buffered.body); err != nil {
			return
		}
	})
}

// etagRecorder keeps the answer in memory until it is known whether it will be sent.
//
// The bodies here are the whole of a listing, so buffering one costs a few kilobytes and
// removes the need to guess at a tag before the body exists.
type etagRecorder struct {
	http.ResponseWriter
	status int
	body   []byte
	header http.Header
}

func (r *etagRecorder) Header() http.Header {
	return r.header
}

func (r *etagRecorder) WriteHeader(status int) {
	r.status = status
}

func (r *etagRecorder) Write(chunk []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	r.body = append(r.body, chunk...)
	return len(chunk), nil
}

// flush hands the buffered answer to the real writer, unchanged.
func (r *etagRecorder) flush() {
	for name, values := range r.header {
		for _, value := range values {
			r.ResponseWriter.Header().Add(name, value)
		}
	}

	status := r.status
	if status == 0 {
		status = http.StatusOK
	}
	r.ResponseWriter.WriteHeader(status)

	if len(r.body) > 0 {
		if _, err := r.ResponseWriter.Write(r.body); err != nil {
			return
		}
	}
}

// tagOf is a quoted strong tag over the body: the first half of a SHA-256, which is plenty
// to tell two versions of a listing apart without making every header long.
func tagOf(body []byte) string {
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

// matchesIfNoneMatch reports whether the request's header names the tag, by value or with
// the "*" wildcard. A list of tags is allowed, so it is split rather than compared whole.
func matchesIfNoneMatch(header, tag string) bool {
	for _, candidate := range strings.Split(header, ",") {
		switch trimmed := strings.TrimSpace(candidate); trimmed {
		case tag, "*":
			return true
		}
	}
	return false
}
