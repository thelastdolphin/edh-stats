package model

// DateLayout is the layout games.played_at uses.
//
// It is shared by everything that reads or writes a game date, so that the storage
// layer, the request payloads and the import tool cannot drift apart.
const DateLayout = "2006-01-02"
