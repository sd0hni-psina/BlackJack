package main

import (
	"log"

	"blackjack/internal/bot"
	"blackjack/internal/config"
	"blackjack/internal/database"
	"blackjack/internal/player"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	db, err := database.New(cfg.DatabasePath)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Database connected")

	playerRepo := player.NewRepository(db.DB)

	b, err := bot.New(cfg, playerRepo)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	if err := b.Run(); err != nil {
		log.Fatalf("Bot error: %v", err)
	}
}
