package main

import (
	"log"

	"example.com/edh-stats/internal/app"
)

func main() {

	cfg := &app.Config{
		DbPath:        "./data/edh_stats.db",
		ServerAddress: ":8080",
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	if err := application.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
}
