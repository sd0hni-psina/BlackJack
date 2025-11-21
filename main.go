package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
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

func getRandomCard() string {
	if len(availableCards) == 0 {
		initDeck()
		fmt.Print("Дилер тасует карты")
	}

	card := availableCards[0]
	availableCards = availableCards[1:]
	return card
}

// Инициализируем колоду и приобвляем карты, что бы получилось 52 карты в колоде
func initDeck() {
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

func init() {
	initDeck()
}

func dealerTurn(dealerCards []string) []string {
	for calculateScore(dealerCards) < 17 {
		dealerCards = append(dealerCards, getRandomCard())
	}
	return dealerCards
}

func winner(playerScore, dealerScore int) string {
	if playerScore > 21 {
		return "💥 Перебор! Вы проиграли!"
	} else if dealerScore > 21 {
		return "🎉 Дилер перебрал! Вы выиграли!"
	} else if playerScore > dealerScore {
		return "🎉 Вы выиграли!"
	} else if playerScore < dealerScore {
		return "😔 Дилер выиграл!"
	} else {
		return "🤝 Ничья!"
	}
}

type Player struct {
	ChatID  int64
	Balance int
	Wins    int
	Losses  int
	Games   int
}

var players = make(map[int64]*Player)

type GameState struct {
	PlayerCards    []string
	DealerCards    []string
	AvailableCards []string
	IsActive       bool
	Bet            int
}

func createDeck() []string {
	deck := []string{}
	cards := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
	for i := 0; i < 4; i++ {
		for _, card := range cards {
			deck = append(deck, card)
		}
	}
	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
	return deck
}

func (g *GameState) drawCard() string {
	if len(g.AvailableCards) == 0 {
		panic("No cards available")
	}
	card := g.AvailableCards[0]
	g.AvailableCards = g.AvailableCards[1:]
	return card
}

var games = make(map[int64]*GameState)

func getOrCreatePlayer(chatID int64) *Player {
	player, exist := players[chatID]
	if !exist {
		player = &Player{
			ChatID:  chatID,
			Balance: 1000,
			Wins:    0,
			Losses:  0,
			Games:   0,
		}
		players[chatID] = player
	}
	return player
}

func isBlackjack(cards []string) bool {
	if len(cards) != 2 {
		return false
	}
	score := calculateScore(cards)
	if score != 21 {
		return false
	}

	hasAce := false
	hasTen := false

	for _, card := range cards {
		if card == "A" {
			hasAce = true
		}
		if card == "10" || card == "J" || card == "Q" || card == "K" {
			hasTen = true
		}
	}
	return hasAce && hasTen
}

func main() {
	godotenv.Load()
	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN environment variable is not set")
	}
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Print("Bot started: %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			callback := update.CallbackQuery
			chatID := callback.Message.Chat.ID
			data := callback.Data

			game := games[chatID]
			if game == nil || !game.IsActive {
				bot.Send(tgbotapi.NewMessage(chatID, "Игра не началась. Используйте /play"))
				continue
			}

			if data == "hit" {
				game.PlayerCards = append(game.PlayerCards, getRandomCard())
				playerScore := calculateScore(game.PlayerCards)

				if playerScore > 21 {
					player := getOrCreatePlayer(chatID)
					player.Losses++
					player.Games++

					message := fmt.Sprintf("🎴 Ваши карты: %v\nОчки: %d\n\n💥 Перебор! Вы проиграли!\n💵 Баланс: %d",
						game.PlayerCards, playerScore, player.Balance)
					game.IsActive = false
					bot.Send(tgbotapi.NewMessage(chatID, message))
				} else {
					message := fmt.Sprintf("🎴 Ваши карты: %v\nОчки: %d\n\n🃏 Карта дилера: %s", game.PlayerCards, playerScore, game.DealerCards[0])

					keyboard := tgbotapi.NewInlineKeyboardMarkup(
						tgbotapi.NewInlineKeyboardRow(
							tgbotapi.NewInlineKeyboardButtonData("Hit", "hit"),
							tgbotapi.NewInlineKeyboardButtonData("Stand", "stand"),
						),
					)
					msg := tgbotapi.NewMessage(chatID, message)
					msg.ReplyMarkup = keyboard
					bot.Send(msg)
				}
			} else if data == "stand" {
				player := getOrCreatePlayer(chatID)

				game.DealerCards = dealerTurn(game.DealerCards)

				playerScore := calculateScore(game.PlayerCards)
				dealerScore := calculateScore(game.DealerCards)

				var result string
				var winAmount int

				if playerScore > 21 {
					result = "💥 Перебор! Вы проиграли!"
					player.Losses++
				} else if dealerScore > 21 {
					result = "🎉 Дилер перебрал! Вы выиграли!"
					winAmount = game.Bet * 2
					player.Balance += winAmount
					player.Wins++
				} else if playerScore > dealerScore {
					result = "🎉 Вы выиграли!"
					winAmount = game.Bet * 2
					player.Balance += winAmount
					player.Wins++
				} else if playerScore < dealerScore {
					result = "😔 Дилер выиграл!"
					player.Losses++
				} else {
					result = "🤝 Ничья! Ставка возвращена"
					player.Balance += game.Bet
				}
				player.Games++
				message := fmt.Sprintf("🎴 Ваши карты: %v (Очки: %d)\n🃏 Карты дилера: %v (Очки: %d)\n\n%s", game.PlayerCards, playerScore, game.DealerCards, dealerScore, result)

				if winAmount > 0 {
					message += fmt.Sprintf("\n💰 Выигрыш: +%d", winAmount)
				}
				message += fmt.Sprintf("\n💵 Баланс: %d", player.Balance)

				game.IsActive = false
				bot.Send(tgbotapi.NewMessage(chatID, message))
			}
			bot.Request(tgbotapi.NewCallback(callback.ID, ""))
		}
		if update.Message == nil {
			continue
		}
		text := update.Message.Text
		chatID := update.Message.Chat.ID

		if strings.HasPrefix(text, "/start") {

			msg := tgbotapi.NewMessage(chatID, "Welcome to Blackjack!")
			bot.Send(msg)
		}
		if strings.HasPrefix(text, "/play") {
			player := getOrCreatePlayer(chatID)

			parts := strings.Fields(text)
			if len(parts) < 2 {
				msg := tgbotapi.NewMessage(chatID, "Укажите ставку: /play 100")
				bot.Send(msg)
				continue
			}

			bet, err := strconv.Atoi(parts[1])
			if err != nil || bet <= 0 {
				msg := tgbotapi.NewMessage(chatID, "Неверная ставка")
				bot.Send(msg)
				continue
			}
			if bet > player.Balance {
				msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Недостадочно средств! Ваш баланас: %d", player.Balance))
				bot.Send(msg)
				continue
			}
			player.Balance -= bet

			deck := createDeck()
			game := &GameState{
				AvailableCards: deck,
				PlayerCards:    []string{},
				DealerCards:    []string{},
				Bet:            bet,
				IsActive:       true,
			}
			game.PlayerCards = append(game.PlayerCards, game.drawCard(), game.drawCard())
			game.DealerCards = append(game.DealerCards, game.drawCard(), game.drawCard())

			games[chatID] = game

			playerScore := calculateScore(game.PlayerCards)

			if isBlackjack(game.PlayerCards) {
				if isBlackjack(game.DealerCards) {
					player.Balance += game.Bet // Ставка возвращается
					message := fmt.Sprintf("🎴 Ваши карты: %v - BLACKJACK!\n🃏 Карты дилера: %v - BLACKJACK!\n\n🤝 Ничья! Ставка возвращена\n💵 Баланс: %d", game.PlayerCards, game.DealerCards, player.Balance)
					game.IsActive = false
					bot.Send(tgbotapi.NewMessage(chatID, message))
					continue
				} else {
					winAmount := int(float64(game.Bet) * 2.5)
					player.Balance += winAmount
					player.Wins++
					player.Games++

					message := fmt.Sprintf("🎴 Ваши карты: %v\n\n🎰 BLACKJACK! 🎰\n\n🃏 Карты дилера: %v\n\n🎉 Вы выиграли!\n💰 Выигрыш: +%d (x2.5)\n💵 Баланс: %d", game.PlayerCards, game.DealerCards, winAmount, player.Balance)
					game.IsActive = false
					bot.Send(tgbotapi.NewMessage(chatID, message))
					continue
				}
			}

			if isBlackjack(game.DealerCards) {
				player.Losses++
				player.Games++
				message := fmt.Sprintf("🎴 Ваши карты: %v (Очки: %d)\n🃏 Карты дилера: %v\n\n🎰 BLACKJACK у дилера! 🎰\n\n😔 Вы проиграли!\n💵 Баланс: %d",
					game.PlayerCards, playerScore, game.DealerCards, player.Balance)
				game.IsActive = false
				bot.Send(tgbotapi.NewMessage(chatID, message))
				continue
			}

			message := fmt.Sprintf("💰 Ставка: %d\n💵 Баланс: %d\n\n🎴 Ваши карты: %v\nОчки: %d\n\n🃏 Карта дилера: %s", bet, player.Balance, game.PlayerCards, playerScore, game.DealerCards[0])

			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("👊 Hit", "hit"),
					tgbotapi.NewInlineKeyboardButtonData("✋ Stand", "stand"),
				),
			)
			msg := tgbotapi.NewMessage(chatID, message)
			msg.ReplyMarkup = keyboard
			bot.Send(msg)
		}
		if strings.HasPrefix(text, "/balance") {
			player := getOrCreatePlayer(chatID)
			message := fmt.Sprintf("💰 Ваш баланс: %d\n🎮 Игр сыграно: %d\n✅ Побед: %d\n❌ Поражений: %d", player.Balance, player.Games, player.Wins, player.Losses)
			msg := tgbotapi.NewMessage(chatID, message)
			bot.Send(msg)
		}
	}

}

// game := &GameState{
// 	PlayerCards: []string{getRandomCard(), getRandomCard()},
// 	DealerCards: []string{getRandomCard(), getRandomCard()},
// 	IsActive:    true,
// }
// games[chatID] = game

// playerScore := calculateScore(game.PlayerCards)
// message := fmt.Sprintf("🎴 Ваши карты: %v\nОчки: %d\n\n 🃏 Карта дилера: %s", game.PlayerCards, playerScore, game.DealerCards[0])

// keyboard := tgbotapi.NewInlineKeyboardMarkup(
// 	tgbotapi.NewInlineKeyboardRow(
// 		tgbotapi.NewInlineKeyboardButtonData("Hit", "hit"),
// 		tgbotapi.NewInlineKeyboardButtonData("Stand", "stand"),
// 	),
// )
// msg := tgbotapi.NewMessage(chatID, message)
// msg.ReplyMarkup = keyboard
// bot.Send(msg)

// var playerCards = []string{}
// var dealerCards = []string{}
// playerCards = append(playerCards, getRandomCard())
// playerCards = append(playerCards, getRandomCard())
// dealerCards = append(dealerCards, getRandomCard())
// dealerCards = append(dealerCards, getRandomCard())

// playerScore := calculateScore(playerCards)
// dealerScore := calculateScore(dealerCards)
// playerCards = playerTurn(playerCards)
// dealerCards = dealerTurn(dealerCards)

// winner(playerScore, dealerScore)

// var playerCards = []string{}
// var dealerCards = []string{}

// func playerTurn(playerCards []string) []string {
// 	for {
// 		score := calculateScore(playerCards)
// 		fmt.Println("Youre cards: ", playerCards, " Score: ", score)

// 		if score > 21 {
// 			fmt.Println("Bust!")
// 			return playerCards
// 		}

// 		fmt.Println("Hit or Stand ?")
// 		var choise string
// 		fmt.Scan(&choise)
// 		if choise == "H" {
// 			playerCards = append(playerCards, getRandomCard())
// 		} else if choise == "S" {
// 			break
// 		} else {
// 			fmt.Println("Invalid choice")
// 		}
// 	}
// 	return playerCards
// }
// time.Sleep(2000 * time.Millisecond)
// fmt.Println("Dealer reveals: ", dealerCards)
// time.Sleep(2000 * time.Millisecond)

// for calculateScore(dealerCards) < 17 {
// 	newCard := getRandomCard()
// 	dealerCards = append(dealerCards, newCard)
// 	fmt.Println("Dealer takes: ", newCard)
// 	time.Sleep(500 * time.Millisecond)
// }
// fmt.Println("Dealer's final hand:", dealerCards, "Score: ", calculateScore(dealerCards))
// time.Sleep(500 * time.Millisecond)
// return dealerCards
//
// обработка при stand
// game.DealerCards = dealerTurn(game.DealerCards)

// playerScore := calculateScore(game.PlayerCards)
// dealerScore := calculateScore(game.DealerCards)

// result := winner(playerScore, dealerScore)

// message := fmt.Sprintf("🎴 Ваши карты: %v (Очки: %d)\n🃏 Карты дилера: %v (Очки: %d)\n\n%s", game.PlayerCards, playerScore, game.DealerCards, dealerScore, result)

// game.IsActive = false
// bot.Send(tgbotapi.NewMessage(chatID, message))
