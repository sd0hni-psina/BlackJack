package game

import "sync"

type Result int

const (
	ResultNone Result = iota
	ResultPlayerWin
	ResultDealerWin
	ResultPush
	ResultBlackjack
)

type State struct {
	PlayerCards []string
	DealerCards []string
	Deck        *Deck
	Bet         int
	IsActive    bool
	CanDouble   bool
}

func NewState(bet int) *State {
	s := &State{
		Deck:        NewDeck(),
		PlayerCards: make([]string, 0, 10),
		DealerCards: make([]string, 0, 10),
		Bet:         bet,
		IsActive:    true,
		CanDouble:   true,
	}

	s.PlayerCards = append(s.PlayerCards, s.Deck.Draw(), s.Deck.Draw())
	s.DealerCards = append(s.DealerCards, s.Deck.Draw(), s.Deck.Draw())

	return s
}

func (s *State) Hit() string {
	card := s.Deck.Draw()
	s.PlayerCards = append(s.PlayerCards, card)
	s.CanDouble = false
	return card
}

func (s *State) Double() string {
	s.Bet *= 2
	s.CanDouble = false
	card := s.Deck.Draw()
	s.PlayerCards = append(s.PlayerCards, card)
	return card
}

func (s *State) DealerPlay() {
	for CalculateScore(s.DealerCards) < 17 {
		s.DealerCards = append(s.DealerCards, s.Deck.Draw())
	}
}

func (s *State) PlayerScore() int {
	return CalculateScore(s.PlayerCards)
}

func (s *State) DealerScore() int {
	return CalculateScore(s.DealerCards)
}

func (s *State) Finish() Result {
	s.IsActive = false
	s.DealerPlay()

	playerScore := s.PlayerScore()
	dealerScore := s.DealerScore()

	switch {
	case playerScore > 21:
		return ResultDealerWin
	case dealerScore > 21:
		return ResultPlayerWin
	case playerScore > dealerScore:
		return ResultPlayerWin
	case playerScore < dealerScore:
		return ResultDealerWin
	default:
		return ResultPush
	}
}

// Manager управляет активными играми
type Manager struct {
	games map[int64]*State
	mu    sync.RWMutex
}

func NewManager() *Manager {
	return &Manager{
		games: make(map[int64]*State),
	}
}

func (m *Manager) Get(chatID int64) *State {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.games[chatID]
}

func (m *Manager) Set(chatID int64, state *State) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.games[chatID] = state
}

func (m *Manager) Delete(chatID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.games, chatID)
}
