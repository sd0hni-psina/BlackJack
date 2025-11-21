package game

import (
	"sync"
)

type Result int

const (
	ResultNone Result = iota
	ResultPlayerWin
	ResultDealerWin
	ResultPush
	ResultBlackjack
)

// рука для сплита
type Hand struct {
	Cards     []string
	Bet       int
	IsStand   bool
	IsDouble  bool
	IsBust    bool
	FromSplit bool
	SplitAces bool
}

func NewHand(bet int) *Hand {
	return &Hand{
		Cards: make([]string, 0, 10),
		Bet:   bet,
	}
}

func (h *Hand) Score() int {
	return CalculateScore(h.Cards)
}

func (h *Hand) CanSplit() bool {
	if len(h.Cards) != 2 || h.FromSplit {
		return false
	}
	// проверка карт на одинаковость
	return CardValues[h.Cards[0]] == CardValues[h.Cards[1]]
}

func (h *Hand) CanDouble() bool {
	return len(h.Cards) == 2 && !h.IsDouble && !h.SplitAces
}

func (h *Hand) IsBlackjack() bool {
	// после сплита блэкджек не работает
	if h.FromSplit {
		return false
	}
	return IsBlackjack(h.Cards)
}

// Храним состояние игры
type State struct {
	Hands       []*Hand
	DealerCards []string
	Deck        *Deck
	CurrentHand int
	IsActive    bool
	InitialBet  int
}

func NewState(bet int) *State {
	s := &State{
		Deck:        NewDeck(),
		Hands:       make([]*Hand, 0, 4),
		DealerCards: make([]string, 0, 10),
		CurrentHand: 0,
		IsActive:    true,
		InitialBet:  bet,
	}

	// создаем новую первую руку
	hand := NewHand(bet)
	hand.Cards = append(hand.Cards, s.Deck.Draw(), s.Deck.Draw())
	s.Hands = append(s.Hands, hand)

	s.DealerCards = append(s.DealerCards, s.Deck.Draw(), s.Deck.Draw())

	// s.PlayerCards = append(s.PlayerCards, s.Deck.Draw(), s.Deck.Draw())
	// s.DealerCards = append(s.DealerCards, s.Deck.Draw(), s.Deck.Draw())

	return s
}

// текущая рука
func (s *State) Current() *Hand {
	if s.CurrentHand >= len(s.Hands) {
		return nil
	}
	return s.Hands[s.CurrentHand]
}

// Общая ставка всех рук
func (s *State) TotalBet() int {
	total := 0
	for _, h := range s.Hands {
		total += h.Bet
	}
	return total
}

// hit для текущей руки
func (s *State) Hit() string {
	hand := s.Current()
	if hand == nil {
		return ""
	}

	card := s.Deck.Draw()
	hand.Cards = append(hand.Cards, card)

	if hand.Score() > 21 {
		hand.IsBust = true
		hand.IsStand = true
	}
	return card
}

// stand чтобы остановиться на текущей руке
func (s *State) Stand() {
	hand := s.Current()
	if hand != nil {
		hand.IsStand = true
	}
}

// double для текущей руки
func (s *State) Double() string {
	hand := s.Current()
	if hand == nil || !hand.CanDouble() {
		return ""
	}

	hand.Bet *= 2
	hand.IsDouble = true

	card := s.Deck.Draw()
	hand.Cards = append(hand.Cards, card)

	if hand.Score() > 21 {
		hand.IsBust = true
	}
	hand.IsStand = true

	return card
}

// split
func (s *State) Split() bool {
	hand := s.Current()
	if hand == nil || !hand.CanSplit() {
		return false
	}

	// вторая карта
	secondCard := hand.Cards[1]
	isAces := hand.Cards[0] == "A"

	// первая карта в текущей руке
	hand.Cards = []string{hand.Cards[0]}
	hand.FromSplit = true
	hand.SplitAces = isAces

	// новая рука для второй карты
	newHand := NewHand(hand.Bet)
	newHand.Cards = []string{secondCard}
	newHand.FromSplit = true
	hand.SplitAces = isAces

	//добираем по карте в каждую руку
	hand.Cards = append(hand.Cards, s.Deck.Draw())
	newHand.Cards = append(newHand.Cards, s.Deck.Draw())

	s.Hands = append(s.Hands[:s.CurrentHand+1], append([]*Hand{newHand}, s.Hands[s.CurrentHand+1:]...)...)

	if isAces {
		hand.IsStand = true
		newHand.IsStand = true
	}

	return true
}

// переход на следующую руку
func (s *State) NextHand() bool {
	s.CurrentHand++

	for s.CurrentHand < len(s.Hands) && s.Hands[s.CurrentHand].IsStand {
		s.CurrentHand++
	}
	return s.CurrentHand < len(s.Hands)
}

// проверка что все руки завершены
func (s *State) AllHandsComplete() bool {
	for _, h := range s.Hands {
		if !h.IsStand {
			return false
		}
	}
	return true
}

func (s *State) DealerPlay() {
	allBust := true
	for _, h := range s.Hands {
		if !h.IsBust {
			allBust = false
			break
		}
	}
	if allBust {
		return
	}

	for CalculateScore(s.DealerCards) < 17 {
		s.DealerCards = append(s.DealerCards, s.Deck.Draw())
	}
}

func (s *State) DealerScore() int {
	return CalculateScore(s.DealerCards)
}

func (s *State) HandResult(hand *Hand) (Result, int) {
	if hand.IsBust {
		return ResultDealerWin, 0
	}

	dealerScore := s.DealerScore()
	playerScore := hand.Score()

	if dealerScore > 21 {
		return ResultPlayerWin, hand.Bet * 2
	}

	if playerScore > dealerScore {
		return ResultPlayerWin, hand.Bet * 2
	} else if playerScore < dealerScore {
		return ResultDealerWin, 0
	}
	return ResultPush, hand.Bet
}

func (s *State) Finish() {
	s.IsActive = false
	s.DealerPlay()
}

func (s *State) PlayerCards() []string {
	if len(s.Hands) == 0 {
		return nil
	}
	return s.Hands[0].Cards
}

func (s *State) PlayerScore() int {
	if len(s.Hands) == 0 {
		return 0
	}
	return s.Hands[0].Score()
}

func (s *State) CanSplit() bool {
	hand := s.Current()
	return hand != nil && hand.CanSplit()
}

func (s *State) CanDouble() bool {
	hand := s.Current()
	return hand != nil && hand.CanDouble()
}

func (s *State) HasMultipleHands() bool {
	return len(s.Hands) > 1
}

// func (s *State) Finish() Result {
// 	s.IsActive = false
// 	s.DealerPlay()

// 	playerScore := s.PlayerScore()
// 	dealerScore := s.DealerScore()

// 	switch {
// 	case playerScore > 21:
// 		return ResultDealerWin
// 	case dealerScore > 21:
// 		return ResultPlayerWin
// 	case playerScore > dealerScore:
// 		return ResultPlayerWin
// 	case playerScore < dealerScore:
// 		return ResultDealerWin
// 	default:
// 		return ResultPush
// 	}
// }

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
