package game

import "math/rand"

var CardValues = map[string]int{
	"2": 2, "3": 3, "4": 4, "5": 5, "6": 6, "7": 7, "8": 8, "9": 9, "10": 10,
	"J": 10, "Q": 10, "K": 10, "A": 11,
}

var cardNames = []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}

type Deck struct {
	cards []string
}

func NewDeck() *Deck {
	d := &Deck{
		cards: make([]string, 0, 52),
	}

	for i := 0; i < 4; i++ {
		d.cards = append(d.cards, cardNames...)
	}

	d.Shuffle()
	return d
}

func (d *Deck) Shuffle() {
	rand.Shuffle(len(d.cards), func(i, j int) {
		d.cards[i], d.cards[j] = d.cards[j], d.cards[i]
	})
}

func (d *Deck) Draw() string {
	if len(d.cards) == 0 {
		*d = *NewDeck()
	}

	card := d.cards[0]
	d.cards = d.cards[1:]
	return card
}

func (d *Deck) Remaining() int {
	return len(d.cards)
}
