package database

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	*sql.DB
}

func New(path string) (*DB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err = migrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate: %w", err)
	}

	return &DB{db}, nil
}

func migrate(db *sql.DB) error {
	schema := `
	CREATE TABLE IF NOT EXISTS players (
		chat_id INTEGER PRIMARY KEY,
		balance INTEGER DEFAULT 1000,
		wins INTEGER DEFAULT 0,
		losses INTEGER DEFAULT 0,
		draws INTEGER DEFAULT 0,
		games INTEGER DEFAULT 0,
		last_bet INTEGER DEFAULT 100,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_players_balance ON players(balance);
	CREATE INDEX IF NOT EXISTS idx_players_games ON players(games);
	`

	_, err := db.Exec(schema)
	return err
}
