package storage

import (
	"context"
	"database/sql"
	"os"
	"testing"

	"example.com/edh-stats/internal/app/model"
	_ "github.com/mattn/go-sqlite3"
)

// Вспомогательная функция для создания временной БД
func setupTestDB(t *testing.T) *DeckRepository {
	// Создаём временный файл БД
	tmpFile, err := os.CreateTemp("", "test_db_*.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Remove(tmpFile.Name()) })

	db, err := sql.Open("sqlite3", tmpFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// Накатываем схему (можно из миграций или упрощённую)
	_, err = db.Exec(`
        CREATE TABLE decks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            name TEXT NOT NULL,
            owner TEXT,
            type TEXT,
            UNIQUE(name, owner)
        );
    `)
	if err != nil {
		t.Fatal(err)
	}

	return &DeckRepository{db: db}
}

func TestDeckRepository_Create(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	deck := &model.Deck{
		Name:  "My Deck",
		Owner: "tester",
		Type:  "Commander",
	}

	created, err := repo.Create(ctx, deck)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if created.ID == 0 {
		t.Error("expected ID to be set, got 0")
	}
}

func TestDeckRepository_GetById(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	// Сначала создаём колоду
	deck, _ := repo.Create(ctx, &model.Deck{
		Name:  "Test Deck",
		Owner: "tester",
		Type:  "Standard",
	})

	// Теперь получаем её по ID
	got, err := repo.GetById(ctx, int(deck.ID))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Name != "Test Deck" {
		t.Errorf("expected name 'Test Deck', got '%s'", got.Name)
	}
}

func TestDeckRepository_Create_Unique(t *testing.T) {
	repo := setupTestDB(t)
	ctx := context.Background()

	// Сначала создаём колоду
	_, err := repo.Create(ctx, &model.Deck{
		Name:  "Test Deck",
		Owner: "tester",
		Type:  "control",
	})

	if err != nil {
		t.Fatalf("cant create first deck")
	}

	_, err = repo.Create(ctx, &model.Deck{
		Name:  "Test Deck",
		Owner: "tester",
		Type:  "control",
	})

	if err == nil {
		t.Errorf("Name and owner must be unique %v", err)
	}
}
