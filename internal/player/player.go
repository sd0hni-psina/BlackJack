package player

import (
	"database/sql"
	"fmt"
)

type Player struct {
	ChatID  int64
	Balance int
	Wins    int
	Losses  int
	Draws   int
	Games   int
	LastBet int
}

type Stats struct {
	ChatID  int64
	Balance int
	Wins    int
	Games   int
	WinRate float64
}

type Repository interface {
	GetOrCreate(chatID int64, startBalance, defaultBet int) (*Player, error)
	Save(player *Player) error
	GetTopByBalance(limit int) ([]Stats, error)
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) GetOrCreate(chatID int64, startBalance, defaultBet int) (*Player, error) {
	player := &Player{ChatID: chatID}

	err := r.db.QueryRow(`
		SELECT balance, wins, losses, draws, games, last_bet
		FROM players WHERE chat_id = ?
	`, chatID).Scan(
		&player.Balance, &player.Wins, &player.Losses,
		&player.Draws, &player.Games, &player.LastBet,
	)

	if err == sql.ErrNoRows {
		player.Balance = startBalance
		player.LastBet = defaultBet

		_, err = r.db.Exec(`
			INSERT INTO players (chat_id, balance, last_bet)
			VALUES (?, ?, ?)
		`, chatID, player.Balance, player.LastBet)

		if err != nil {
			return nil, fmt.Errorf("failed to create player: %w", err)
		}
		return player, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get player: %w", err)
	}

	return player, nil
}

func (r *SQLiteRepository) Save(player *Player) error {
	_, err := r.db.Exec(`
		UPDATE players SET
			balance = ?, wins = ?, losses = ?, draws = ?,
			games = ?, last_bet = ?, updated_at = CURRENT_TIMESTAMP
		WHERE chat_id = ?
	`, player.Balance, player.Wins, player.Losses, player.Draws,
		player.Games, player.LastBet, player.ChatID)

	if err != nil {
		return fmt.Errorf("failed to save player: %w", err)
	}
	return nil
}

func (r *SQLiteRepository) GetTopByBalance(limit int) ([]Stats, error) {
	rows, err := r.db.Query(`
		SELECT chat_id, balance, wins, games
		FROM players
		WHERE games > 0
		ORDER BY balance DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []Stats
	for rows.Next() {
		var s Stats
		if err := rows.Scan(&s.ChatID, &s.Balance, &s.Wins, &s.Games); err != nil {
			return nil, err
		}
		if s.Games > 0 {
			s.WinRate = float64(s.Wins) / float64(s.Games) * 100
		}
		stats = append(stats, s)
	}

	return stats, rows.Err()
}

func (p *Player) AddWin(winAmount int) {
	p.Balance += winAmount
	p.Wins++
	p.Games++
}

func (p *Player) AddLoss() {
	p.Losses++
	p.Games++
}

func (p *Player) AddDraw(betAmount int) {
	p.Balance += betAmount
	p.Draws++
	p.Games++
}

func (p *Player) PlaceBet(amount int) bool {
	if amount > p.Balance {
		return false
	}
	p.Balance -= amount
	p.LastBet = amount
	return true
}

func (p *Player) CanAfford(amount int) bool {
	return p.Balance >= amount
}

func (p *Player) WinRate() float64 {
	if p.Games == 0 {
		return 0
	}
	return float64(p.Wins) / float64(p.Games) * 100
}
