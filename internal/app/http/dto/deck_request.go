// Package dto holds the request payloads of the HTTP API.
package dto

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Field limits of a deck payload.
const (
	MaxNameLen  = 50
	MaxOwnerLen = 50
	MaxTypeLen  = 50
)

// CreateDeckRequest is the JSON body of POST /decks.
type CreateDeckRequest struct {
	Name  string `json:"name"`
	Owner string `json:"owner"`
	Type  string `json:"type"`
}

// Normalize trims surrounding whitespace so that " Deck " and "Deck" are the
// same deck and cannot be used to slip past the name+owner uniqueness rule.
func (r *CreateDeckRequest) Normalize() {
	r.Name = strings.TrimSpace(r.Name)
	r.Owner = strings.TrimSpace(r.Owner)
	r.Type = strings.TrimSpace(r.Type)
}

// Validate reports the first problem found in the payload.
//
// The length is measured in runes, not bytes: the database columns are limited
// in characters, and a 50-rune Cyrillic name is a legal 50-character value.
func (r CreateDeckRequest) Validate() error {
	if r.Name == "" {
		return errors.New("name is required")
	}
	if utf8.RuneCountInString(r.Name) > MaxNameLen {
		return fmt.Errorf("name must be at most %d characters", MaxNameLen)
	}

	if r.Owner == "" {
		return errors.New("owner is required")
	}
	if utf8.RuneCountInString(r.Owner) > MaxOwnerLen {
		return fmt.Errorf("owner must be at most %d characters", MaxOwnerLen)
	}

	if r.Type == "" {
		return errors.New("type is required")
	}
	if utf8.RuneCountInString(r.Type) > MaxTypeLen {
		return fmt.Errorf("type must be at most %d characters", MaxTypeLen)
	}

	return nil
}
