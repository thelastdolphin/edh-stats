package storage

// Pagination bounds shared by the storage and HTTP layers.
const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
)

// DeckFilters narrows and paginates a deck listing.
type DeckFilters struct {
	Name  string
	Owner string
	Type  string
	Page  int
	Limit int
}

// Clamp returns a copy with Page and Limit forced into the supported range.
//
// The HTTP layer rejects out-of-range values with 400, so this is a second line
// of defence for other callers rather than the primary validation.
func (f DeckFilters) Clamp() DeckFilters {
	if f.Page < 1 {
		f.Page = DefaultPage
	}
	if f.Limit < 1 {
		f.Limit = DefaultLimit
	}
	if f.Limit > MaxLimit {
		f.Limit = MaxLimit
	}
	return f
}

// Offset is the SQL OFFSET for the 1-based Page. Call Clamp first.
func (f DeckFilters) Offset() int {
	return (f.Page - 1) * f.Limit
}
