package main

import (
	"fmt"
	"math/rand"
	"time"
)

// Колода карт 
var deck = map[string]int{
	"2":  2,
	"3":  3,
	"4":  4,
	"5":  5,
	"6":  6,
	"7":  7,
	"8":  8,
	"9":  9,
	"10": 10,
	"J":  10,
	"Q":  10,
	"K":  10,
	"A":  11,
}

// Инициализация колоды карт
var availableCards []string

// Функция считает сколько очков
func calculateScore(hand []string) int {
	score := 0
	aces := 0
	
	for _, card := range hand {
		score += deck[card]
		if card == "A" {
			aces++
		}
	}
	// Если очков больше 21 и есть тузы, то уменьшаем очки на 10 и уменьшаем количество туза на 1
	for score > 21 && aces > 0 {
		score -= 10
		aces--
	}
	return score
}

// Инициализируем колоду и приобвляем карты, что бы получилось 52 карты в колоде
func init() {
	availableCards = []string{}

	cards := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}

	for i := 0; i < 4; i++ {
		for _, card := range cards {
			availableCards = append(availableCards, card)
		}
	}

	rand.Shuffle(len(availableCards), func(i, j int) {
		availableCards[i], availableCards[j] = availableCards[j], availableCards[i]
	})
}

func getRandomCard() string {
	if len(availableCards) == 0 {
		panic("No cards available")
	}

	card := availableCards[0]
	availableCards = availableCards[1:]
	return card
}

// var playerCards = []string{}
// var dealerCards = []string{}

func playerTurn(playerCards []string) []string {
	for {
		score := calculateScore(playerCards)
		fmt.Println("Youre cards: ", playerCards, " Score: ", score)

		if score > 21 {
			fmt.Println("Bust!")
			return playerCards
		}

		fmt.Println("Hit or Stand ?")
		var choise string
		fmt.Scan(&choise)
		if choise == "H" {
			playerCards = append(playerCards, getRandomCard())
		} else if choise == "S" {
			break
		} else {
			fmt.Println("Invalid choice")
		}
	}
	return playerCards
}

func dealerTurn(dealerCards []string) []string {
	time.Sleep(2000 * time.Millisecond)
	fmt.Println("Dealer reveals: ", dealerCards)
	time.Sleep(2000 * time.Millisecond)

	for calculateScore(dealerCards) < 17 {
		newCard := getRandomCard()
		dealerCards = append(dealerCards, newCard)
		fmt.Println("Dealer takes: ", newCard)
		time.Sleep(500 * time.Millisecond)
	}
	fmt.Println("Dealer's final hand:", dealerCards, "Score: ", calculateScore(dealerCards))
	time.Sleep(500 * time.Millisecond)
	return dealerCards
}

func winner(playerScore, dealerScore int) {
	if playerScore > 21 {
	} else if dealerScore > 21 {
		fmt.Println("Dealer busts! You Win!")
	} else if playerScore > dealerScore {
		fmt.Println("You WIN!")
	} else if playerScore < dealerScore {
		fmt.Println("Dealer Wins!")
	} else if playerScore == dealerScore {
		fmt.Println("Push (tie)!")
	}
	time.Sleep(500 * time.Millisecond)
}
func main() {
	var playerCards = []string{}
	var dealerCards = []string{}
	playerCards = append(playerCards, getRandomCard())
	playerCards = append(playerCards, getRandomCard())
	dealerCards = append(dealerCards, getRandomCard())
	dealerCards = append(dealerCards, getRandomCard())

	playerScore := calculateScore(playerCards)
	dealerScore := calculateScore(dealerCards)
	playerCards = playerTurn(playerCards)
	dealerCards = dealerTurn(dealerCards)

	winner(playerScore, dealerScore)
}
