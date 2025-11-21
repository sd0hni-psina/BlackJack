package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken      string
	DatabasePath  string
	StartBalance  int
	DefaultBet    int
	MinBet        int
	MaxBet        int
	BlackjackPays float64
}

func Load() (*Config, error) {
	godotenv.Load()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("BOT_TOKEN is not set")
	}

	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "./blackjack.db"
	}

	return &Config{
		BotToken:      token,
		DatabasePath:  dbPath,
		StartBalance:  1000,
		DefaultBet:    100,
		MinBet:        10,
		MaxBet:        10000,
		BlackjackPays: 2.5,
	}, nil
}
