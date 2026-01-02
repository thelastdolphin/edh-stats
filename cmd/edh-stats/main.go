package main

import (
	"log"

	"example.com/edh-stats/internal/app"
)

func main() {

	cfg := &app.Config{
		Dbstring:      "postgresql://tld94:m1sf1ts@localhost:5432/edhstats?sslmode=disable",
		ServerAddress: "localhost:8080",
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("Failed to create application: %v", err)
	}

	if err := application.Start(); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
}
