package http

// Error codes of the API.
//
// The code is the machine-readable half of an error response: a client reacts to
// it, while the message explains the problem to a human and may be reworded at any
// time. As with the rest of the contract, a code is never renamed and never reused
// for a different situation - a new situation gets a new code.
const (
	// CodeInvalidID is returned when a path parameter is not a usable identifier.
	CodeInvalidID = "invalid_id"

	// CodeInvalidQuery is returned when a query parameter cannot be used.
	CodeInvalidQuery = "invalid_query"

	// CodeInvalidJSON is returned when the body is not a single JSON object.
	CodeInvalidJSON = "invalid_json"

	// CodeBodyTooLarge is returned when the body exceeds the accepted size.
	CodeBodyTooLarge = "body_too_large"

	// CodeValidationFailed is returned when the payload is well-formed but wrong.
	CodeValidationFailed = "validation_failed"

	// CodeDeckNotFound is returned when no deck has the requested identifier.
	CodeDeckNotFound = "deck_not_found"

	// CodeGameNotFound is returned when no game has the requested identifier.
	CodeGameNotFound = "game_not_found"

	// CodeDeckDuplicate is returned when the name and owner are already taken.
	CodeDeckDuplicate = "deck_duplicate"

	// CodeDeckInUse is returned when a game still refers to the deck.
	CodeDeckInUse = "deck_in_use"

	// CodeUnknownDeck is returned when a payload refers to a deck that does not exist.
	CodeUnknownDeck = "unknown_deck"

	// CodeDuplicateParticipant is returned when one deck appears twice in a game.
	CodeDuplicateParticipant = "duplicate_participant"

	// CodeUnauthorized is returned when the request carries no valid bearer token.
	CodeUnauthorized = "unauthorized"

	// CodeInternal is returned when the failure is on the server side.
	CodeInternal = "internal_error"
)
