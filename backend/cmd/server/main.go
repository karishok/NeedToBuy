// Command server runs the NeedToBuy HTTP API.
package main

import (
	"log"
	"net/http"

	"needtobuy/internal/config"
	"needtobuy/internal/db"
	"needtobuy/internal/httpapi"
)

func main() {
	cfg := config.Load()

	if err := db.Migrate(cfg.DatabaseURL); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	database, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer database.Close()

	router := httpapi.NewRouter(database)

	log.Printf("listening on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
