package storage

import (
	"example.com/edh-stats/internal/app/model"
	"testing"
)

func TestCreatePlayer(t *testing.T) {
	var testPlayer = model.Player{
		ID:         123,
		PlayerName: "TestPlayer",
	}

	testDbConfig := ""
	testPgxInterface := &DbMethods{}
	testPlayerRepository := PlayerRepository{}
	testDeckRepository := DeckRepository{}
	testStore := Store{
		dbConfig:         &testDbConfig,
		db:               testPgxInterface,
		playerRepository: &testPlayerRepository,
		deckRepository:   &testDeckRepository,
	}
	_, err := testPlayerRepository.Create(&testPlayer, &testStore)
	if err != nil {
	}
}
