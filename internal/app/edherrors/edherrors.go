// Package edherrors holds the domain errors shared by the storage and HTTP
// layers.
//
// They live in their own package so that handlers can map a failure to a status
// code without importing driver- or SQL-specific errors, and so that the
// storage layer never has to know about HTTP.
package edherrors

import "errors"

var (
	// ErrDeckNotFound is returned when a deck lookup matched no row.
	ErrDeckNotFound = errors.New("deck not found")

	// ErrDeckDuplicate is returned when a deck with the same name and owner
	// already exists.
	ErrDeckDuplicate = errors.New("deck already exists")
)
