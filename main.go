package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	_ "github.com/mattn/go-sqlite3"
)

var availableCards []string
var players = make(map[int64]*Player)
var games = make(map[int64]*GameState)
var db *sql.DB

// Состояние игрока
type Player struct {
	ChatID  int64
	Balance int
	Wins    int
	Losses  int
	Draws   int
	Games   int
	LastBet int
}

// Состояние игры
type GameState struct {
	PlayerCards    []string
	DealerCards    []string
	AvailableCards []string
	IsActive       bool
	Bet            int
	CanDouble      bool
	//CanSplit       bool Скоро че нить придумаю
}

// Значение каждой карты
var cardValues = map[string]int{
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

func initDB() error {
	var err error
	db, err = sql.Open("sqlite3", "./blackjack.db")
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	if err = db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	createTableSQL := `
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

	CREATE INDEX IF NOT EXISTS idx_players ON players(games);
	`
	_, err = db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	log.Println("Tables created successfully")
	return nil
}

// Функция подсчитывает очки
func calculateScore(hand []string) int {
	score := 0
	aces := 0

	for _, card := range hand {
		score += cardValues[card]
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

// Функция создает колоду карт и перешивает ее
func createDeck() []string {
	cards := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}
	deck := make([]string, 0, 52)

	for i := 0; i < 4; i++ {
		deck = append(deck, cards...)
	}

	rand.Shuffle(len(deck), func(i, j int) {
		deck[i], deck[j] = deck[j], deck[i]
	})
	return deck
}

func (g *GameState) drawCard() string {
	if len(g.AvailableCards) == 0 {
		g.AvailableCards = createDeck()
	}
	card := g.AvailableCards[0]
	g.AvailableCards = g.AvailableCards[1:]
	return card
}

// Ход дилера
func dealerTurn(game *GameState) {
	for calculateScore(game.DealerCards) < 17 {
		game.DealerCards = append(game.DealerCards, game.drawCard())
	}
}

// Проверка на BlackJack
func isBlackjack(cards []string) bool {
	if len(cards) != 2 {
		return false
	}
	if calculateScore(cards) != 21 {
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

// Получаем или создаем нового игрока
func getOrCreatePlayer(chatID int64) (*Player, error) {
	player := &Player{ChatID: chatID}

	err := db.QueryRow(`
			SELECT balance, wins, losses, draws, games, last_bet
			FROM players WHERE chat_id = ?
		`, chatID).Scan(
		&player.Balance, &player.Wins, &player.Losses,
		&player.Draws, &player.Games, &player.LastBet,
	)

	if err == sql.ErrNoRows {
		// Создаём нового игрока
		player.Balance = 1000
		player.LastBet = 100

		_, err = db.Exec(`
				INSERT INTO players (chat_id, balance, last_bet)
				VALUES (?, ?, ?)
			`, chatID, player.Balance, player.LastBet)

		if err != nil {
			return nil, fmt.Errorf("failed to create player: %w", err)
		}
		log.Printf("New player created: %d", chatID)
		return player, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get player: %w", err)
	}

	return player, nil
}

// save player to DB
func savePlayer(player *Player) error {
	_, err := db.Exec(`
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

func getGameKeyboard(canDouble bool) tgbotapi.InlineKeyboardMarkup {
	row := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("👊 Hit", "hit"),
		tgbotapi.NewInlineKeyboardButtonData("✋ Stand", "stand"),
	}
	if canDouble {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("💰 Double", "double"))
	}
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func getEndGameKeyboard(lastBet int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("🔄 Играть ещё (%d)", lastBet), "play_again"),
			tgbotapi.NewInlineKeyboardButtonData("💵 Баланс", "balance"),
		),
	)
}

// Форматирование результата игры
func formatGameEnd(game *GameState, player *Player, result string, winAmount int) string {
	playerScore := calculateScore(game.PlayerCards)
	dealerScore := calculateScore(game.DealerCards)

	msg := fmt.Sprintf("🎴 Ваши карты: %v (Очки: %d)\n🃏 Карты дилера: %v (Очки: %d)\n\n%s",
		game.PlayerCards, playerScore, game.DealerCards, dealerScore, result)

	if winAmount > 0 {
		msg += fmt.Sprintf("\n💰 Выигрыш: +%d", winAmount)
	}
	msg += fmt.Sprintf("\n💵 Баланс: %d", player.Balance)

	return msg
}

// Завершение игры и определение победителя
func finishGame(game *GameState, player *Player) (string, int) {
	dealerTurn(game)

	playerScore := calculateScore(game.PlayerCards)
	dealerScore := calculateScore(game.DealerCards)

	var result string
	var winAmount int

	switch {
	case playerScore > 21:
		result = "💥 Перебор! Вы проиграли!"
		player.Losses++
	case dealerScore > 21:
		result = "🎉 Дилер перебрал! Вы выиграли!"
		winAmount = game.Bet * 2
		player.Balance += winAmount
		player.Wins++
	case playerScore > dealerScore:
		result = "🎉 Вы выиграли!"
		winAmount = game.Bet * 2
		player.Balance += winAmount
		player.Wins++
	case playerScore < dealerScore:
		result = "😔 Дилер выиграл!"
		player.Losses++
	default:
		result = "🤝 Ничья! Ставка возвращена"
		player.Balance += game.Bet
		player.Draws++
	}

	player.Games++
	game.IsActive = false

	if err := savePlayer(player); err != nil {
		log.Printf("Ошибка сохранения игрока: %v", err)
	}

	return result, winAmount
}

// Начать новую игру
func startGame(chatID int64, bet int, bot *tgbotapi.BotAPI) {
	player, err := getOrCreatePlayer(chatID)
	if err != nil {
		log.Printf("Ошибка получения или создания игрока: %v", err)
		bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка. Попробуйте позже."))
		return
	}

	if bet > player.Balance {
		bot.Send(tgbotapi.NewMessage(chatID,
			fmt.Sprintf("❌ Недостаточно средств! Баланс: %d", player.Balance)))
		return
	}

	player.Balance -= bet
	player.LastBet = bet

	game := &GameState{
		AvailableCards: createDeck(),
		PlayerCards:    []string{},
		DealerCards:    []string{},
		Bet:            bet,
		IsActive:       true,
		CanDouble:      true,
	}

	game.PlayerCards = append(game.PlayerCards, game.drawCard(), game.drawCard())
	game.DealerCards = append(game.DealerCards, game.drawCard(), game.drawCard())
	games[chatID] = game

	playerScore := calculateScore(game.PlayerCards)

	// Проверка на Blackjack
	if isBlackjack(game.PlayerCards) {
		if isBlackjack(game.DealerCards) {
			player.Balance += game.Bet
			player.Draws++
			player.Games++
			game.IsActive = false
			savePlayer(player)

			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
				"🎴 Ваши карты: %v - BLACKJACK!\n🃏 Карты дилера: %v - BLACKJACK!\n\n🤝 Ничья!\n💵 Баланс: %d",
				game.PlayerCards, game.DealerCards, player.Balance))
			msg.ReplyMarkup = getEndGameKeyboard(player.LastBet)
			bot.Send(msg)
			return
		}

		winAmount := int(float64(game.Bet) * 2.5)
		player.Balance += winAmount
		player.Wins++
		player.Games++
		game.IsActive = false
		savePlayer(player)

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"🎴 Ваши карты: %v\n\n🎰 BLACKJACK! 🎰\n\n💰 Выигрыш: +%d (x2.5)\n💵 Баланс: %d",
			game.PlayerCards, winAmount, player.Balance))
		msg.ReplyMarkup = getEndGameKeyboard(player.LastBet)
		bot.Send(msg)
		return
	}

	if isBlackjack(game.DealerCards) {
		player.Losses++
		player.Games++
		game.IsActive = false
		savePlayer(player)

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"🎴 Ваши карты: %v (Очки: %d)\n🃏 Карты дилера: %v\n\n🎰 BLACKJACK у дилера!\n😔 Вы проиграли!\n💵 Баланс: %d",
			game.PlayerCards, playerScore, game.DealerCards, player.Balance))
		msg.ReplyMarkup = getEndGameKeyboard(player.LastBet)
		bot.Send(msg)
		return
	}
	savePlayer(player)

	// Можно ли удвоить
	canDouble := player.Balance >= game.Bet

	message := fmt.Sprintf("💰 Ставка: %d\n💵 Баланс: %d\n\n🎴 Ваши карты: %v\nОчки: %d\n\n🃏 Карта дилера: %s",
		bet, player.Balance, game.PlayerCards, playerScore, game.DealerCards[0])

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ReplyMarkup = getGameKeyboard(canDouble)
	bot.Send(msg)
}

func handleCallback(bot *tgbotapi.BotAPI, callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	data := callback.Data

	player, err := getOrCreatePlayer(chatID)
	if err != nil {
		log.Printf("Error: %v", err)
		bot.Request(tgbotapi.NewCallback(callback.ID, "Ошибка"))
		return
	}

	// Играть ещё
	if data == "play_again" {
		bot.Request(tgbotapi.NewCallback(callback.ID, ""))
		startGame(chatID, player.LastBet, bot)
		return
	}

	// Баланс
	if data == "balance" {
		bot.Request(tgbotapi.NewCallback(callback.ID,
			fmt.Sprintf("💵 %d", player.Balance)))
		return
	}

	game := games[chatID]
	if game == nil || !game.IsActive {
		bot.Request(tgbotapi.NewCallback(callback.ID, "Игра не активна"))
		return
	}

	switch data {
	case "hit":
		game.PlayerCards = append(game.PlayerCards, game.drawCard())
		game.CanDouble = false
		playerScore := calculateScore(game.PlayerCards)

		if playerScore > 21 {
			player.Losses++
			player.Games++
			game.IsActive = false
			savePlayer(player)

			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
				"🎴 Вы: %v (%d)\n\n💥 Перебор!\n💵 Баланс: %d",
				game.PlayerCards, playerScore, player.Balance))
			msg.ReplyMarkup = getEndGameKeyboard(player.LastBet)
			bot.Send(msg)
		} else {
			message := fmt.Sprintf("🎴 Вы: %v (%d)\n🃏 Дилер: [%s, ?]",
				game.PlayerCards, playerScore, game.DealerCards[0])

			msg := tgbotapi.NewMessage(chatID, message)
			msg.ReplyMarkup = getGameKeyboard(false)
			bot.Send(msg)
		}

	case "stand":
		result, winAmount := finishGame(game, player)
		msg := tgbotapi.NewMessage(chatID, formatGameEnd(game, player, result, winAmount))
		msg.ReplyMarkup = getEndGameKeyboard(player.LastBet)
		bot.Send(msg)

	case "double":
		if !game.CanDouble || player.Balance < game.Bet {
			bot.Request(tgbotapi.NewCallback(callback.ID, "Недоступно"))
			return
		}

		player.Balance -= game.Bet
		game.Bet *= 2
		game.CanDouble = false
		game.PlayerCards = append(game.PlayerCards, game.drawCard())

		playerScore := calculateScore(game.PlayerCards)

		if playerScore > 21 {
			player.Losses++
			player.Games++
			game.IsActive = false
			savePlayer(player)

			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
				"💰 Удвоено: %d\n\n🎴 Вы: %v (%d)\n\n💥 Перебор!\n💵 Баланс: %d",
				game.Bet, game.PlayerCards, playerScore, player.Balance))
			msg.ReplyMarkup = getEndGameKeyboard(player.LastBet)
			bot.Send(msg)
		} else {
			result, winAmount := finishGame(game, player)
			message := fmt.Sprintf("💰 Удвоено: %d\n\n%s",
				game.Bet, formatGameEnd(game, player, result, winAmount))

			msg := tgbotapi.NewMessage(chatID, message)
			msg.ReplyMarkup = getEndGameKeyboard(player.LastBet)
			bot.Send(msg)
		}
	}

	bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
	chatID := message.Chat.ID
	text := message.Text

	switch {
	case strings.HasPrefix(text, "/start"):
		player, _ := getOrCreatePlayer(chatID)
		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"🎰 Добро пожаловать в Blackjack!\n\n"+
				"💵 Баланс: %d\n\n"+
				"/play <ставка> — играть\n"+
				"/balance — статистика\n"+
				"/top — топ игроков",
			player.Balance))
		bot.Send(msg)

	case strings.HasPrefix(text, "/play"):
		parts := strings.Fields(text)
		bet := 100

		if len(parts) >= 2 {
			if b, err := strconv.Atoi(parts[1]); err == nil && b > 0 {
				bet = b
			} else {
				bot.Send(tgbotapi.NewMessage(chatID, "❌ Пример: /play 100"))
				return
			}
		}

		startGame(chatID, bet, bot)

	case strings.HasPrefix(text, "/balance"):
		player, err := getOrCreatePlayer(chatID)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка"))
			return
		}

		winRate := 0.0
		if player.Games > 0 {
			winRate = float64(player.Wins) / float64(player.Games) * 100
		}

		msg := tgbotapi.NewMessage(chatID, fmt.Sprintf(
			"💰 Баланс: %d\n\n"+
				"📊 Статистика:\n"+
				"🎮 Игр: %d\n"+
				"✅ Побед: %d (%.1f%%)\n"+
				"❌ Поражений: %d\n"+
				"🤝 Ничьих: %d",
			player.Balance, player.Games, player.Wins, winRate, player.Losses, player.Draws))
		bot.Send(msg)

	case strings.HasPrefix(text, "/top"):
		rows, err := db.Query(`
			SELECT chat_id, balance, wins, games
			FROM players
			WHERE games > 0
			ORDER BY balance DESC
			LIMIT 10
		`)
		if err != nil {
			bot.Send(tgbotapi.NewMessage(chatID, "❌ Ошибка"))
			return
		}
		defer rows.Close()

		var top strings.Builder
		top.WriteString("🏆 Топ игроков:\n\n")

		place := 1
		for rows.Next() {
			var id int64
			var balance, wins, games int
			rows.Scan(&id, &balance, &wins, &games)

			medal := ""
			switch place {
			case 1:
				medal = "🥇"
			case 2:
				medal = "🥈"
			case 3:
				medal = "🥉"
			default:
				medal = fmt.Sprintf("%d.", place)
			}

			winRate := float64(wins) / float64(games) * 100
			top.WriteString(fmt.Sprintf("%s %d 💰 | %d игр (%.0f%%)\n",
				medal, balance, games, winRate))
			place++
		}

		if place == 1 {
			top.WriteString("Пока никто не играл!")
		}

		bot.Send(tgbotapi.NewMessage(chatID, top.String()))
	}
}

func main() {
	godotenv.Load()

	// Инициализация БД
	if err := initDB(); err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	token := os.Getenv("BOT_TOKEN")
	if token == "" {
		log.Fatal("BOT_TOKEN is not set")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Bot started: @%s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			go handleCallback(bot, update.CallbackQuery)
			continue
		}

		if update.Message != nil {
			go handleMessage(bot, update.Message)
		}
	}
}

// godotenv.Load()
// token := os.Getenv("BOT_TOKEN")
// if token == "" {
// 	log.Fatal("BOT_TOKEN environment variable is not set")
// }
// bot, err := tgbotapi.NewBotAPI(token)
// if err != nil {
// 	log.Panic(err)
// }

// bot.Debug = true

// log.Print("Bot started: %s", bot.Self.UserName)

// u := tgbotapi.NewUpdate(0)
// u.Timeout = 60

// updates := bot.GetUpdatesChan(u)

// for update := range updates {
// 	if update.CallbackQuery != nil {
// 		callback := update.CallbackQuery
// 		chatID := callback.Message.Chat.ID
// 		data := callback.Data

// 		game := games[chatID]
// 		if game == nil || !game.IsActive {
// 			bot.Send(tgbotapi.NewMessage(chatID, "Игра не началась. Используйте /play"))
// 			continue
// 		}

// 		if data == "hit" {
// 			game.PlayerCards = append(game.PlayerCards, getRandomCard())
// 			playerScore := calculateScore(game.PlayerCards)

// 			if playerScore > 21 {
// 				player := getOrCreatePlayer(chatID)
// 				player.Losses++
// 				player.Games++

// 				message := fmt.Sprintf("🎴 Ваши карты: %v\nОчки: %d\n\n💥 Перебор! Вы проиграли!\n💵 Баланс: %d",
// 					game.PlayerCards, playerScore, player.Balance)
// 				game.IsActive = false
// 				bot.Send(tgbotapi.NewMessage(chatID, message))
// 			} else {
// 				message := fmt.Sprintf("🎴 Ваши карты: %v\nОчки: %d\n\n🃏 Карта дилера: %s", game.PlayerCards, playerScore, game.DealerCards[0])

// 				keyboard := tgbotapi.NewInlineKeyboardMarkup(
// 					tgbotapi.NewInlineKeyboardRow(
// 						tgbotapi.NewInlineKeyboardButtonData("Hit", "hit"),
// 						tgbotapi.NewInlineKeyboardButtonData("Stand", "stand"),
// 					),
// 				)
// 				msg := tgbotapi.NewMessage(chatID, message)
// 				msg.ReplyMarkup = keyboard
// 				bot.Send(msg)
// 			}
// 		} else if data == "stand" {
// 			player := getOrCreatePlayer(chatID)

// 			game.DealerCards = dealerTurn(game.DealerCards)

// 			playerScore := calculateScore(game.PlayerCards)
// 			dealerScore := calculateScore(game.DealerCards)

// 			var result string
// 			var winAmount int

// 			if playerScore > 21 {
// 				result = "💥 Перебор! Вы проиграли!"
// 				player.Losses++
// 			} else if dealerScore > 21 {
// 				result = "🎉 Дилер перебрал! Вы выиграли!"
// 				winAmount = game.Bet * 2
// 				player.Balance += winAmount
// 				player.Wins++
// 			} else if playerScore > dealerScore {
// 				result = "🎉 Вы выиграли!"
// 				winAmount = game.Bet * 2
// 				player.Balance += winAmount
// 				player.Wins++
// 			} else if playerScore < dealerScore {
// 				result = "😔 Дилер выиграл!"
// 				player.Losses++
// 			} else {
// 				result = "🤝 Ничья! Ставка возвращена"
// 				player.Balance += game.Bet
// 			}
// 			player.Games++
// 			message := fmt.Sprintf("🎴 Ваши карты: %v (Очки: %d)\n🃏 Карты дилера: %v (Очки: %d)\n\n%s", game.PlayerCards, playerScore, game.DealerCards, dealerScore, result)

// 			if winAmount > 0 {
// 				message += fmt.Sprintf("\n💰 Выигрыш: +%d", winAmount)
// 			}
// 			message += fmt.Sprintf("\n💵 Баланс: %d", player.Balance)

// 			game.IsActive = false
// 			bot.Send(tgbotapi.NewMessage(chatID, message))
// 		}
// 		bot.Request(tgbotapi.NewCallback(callback.ID, ""))
// 	}
// 	if update.Message == nil {
// 		continue
// 	}
// 	text := update.Message.Text
// 	chatID := update.Message.Chat.ID

// 	if strings.HasPrefix(text, "/start") {

// 		msg := tgbotapi.NewMessage(chatID, "Welcome to Blackjack!")
// 		bot.Send(msg)
// 	}
// 	if strings.HasPrefix(text, "/play") {
// 		player := getOrCreatePlayer(chatID)

// 		parts := strings.Fields(text)
// 		if len(parts) < 2 {
// 			msg := tgbotapi.NewMessage(chatID, "Укажите ставку: /play 100")
// 			bot.Send(msg)
// 			continue
// 		}

// 		bet, err := strconv.Atoi(parts[1])
// 		if err != nil || bet <= 0 {
// 			msg := tgbotapi.NewMessage(chatID, "Неверная ставка")
// 			bot.Send(msg)
// 			continue
// 		}
// 		if bet > player.Balance {
// 			msg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Недостадочно средств! Ваш баланас: %d", player.Balance))
// 			bot.Send(msg)
// 			continue
// 		}
// 		player.Balance -= bet

// 		deck := createDeck()
// 		game := &GameState{
// 			AvailableCards: deck,
// 			PlayerCards:    []string{},
// 			DealerCards:    []string{},
// 			Bet:            bet,
// 			IsActive:       true,
// 		}
// 		game.PlayerCards = append(game.PlayerCards, game.drawCard(), game.drawCard())
// 		game.DealerCards = append(game.DealerCards, game.drawCard(), game.drawCard())

// 		games[chatID] = game

// 		playerScore := calculateScore(game.PlayerCards)

// 		if isBlackjack(game.PlayerCards) {
// 			if isBlackjack(game.DealerCards) {
// 				player.Balance += game.Bet // Ставка возвращается
// 				message := fmt.Sprintf("🎴 Ваши карты: %v - BLACKJACK!\n🃏 Карты дилера: %v - BLACKJACK!\n\n🤝 Ничья! Ставка возвращена\n💵 Баланс: %d", game.PlayerCards, game.DealerCards, player.Balance)
// 				game.IsActive = false
// 				bot.Send(tgbotapi.NewMessage(chatID, message))
// 				continue
// 			} else {
// 				winAmount := int(float64(game.Bet) * 2.5)
// 				player.Balance += winAmount
// 				player.Wins++
// 				player.Games++

// 				message := fmt.Sprintf("🎴 Ваши карты: %v\n\n🎰 BLACKJACK! 🎰\n\n🃏 Карты дилера: %v\n\n🎉 Вы выиграли!\n💰 Выигрыш: +%d (x2.5)\n💵 Баланс: %d", game.PlayerCards, game.DealerCards, winAmount, player.Balance)
// 				game.IsActive = false
// 				bot.Send(tgbotapi.NewMessage(chatID, message))
// 				continue
// 			}
// 		}

// 		if isBlackjack(game.DealerCards) {
// 			player.Losses++
// 			player.Games++
// 			message := fmt.Sprintf("🎴 Ваши карты: %v (Очки: %d)\n🃏 Карты дилера: %v\n\n🎰 BLACKJACK у дилера! 🎰\n\n😔 Вы проиграли!\n💵 Баланс: %d",
// 				game.PlayerCards, playerScore, game.DealerCards, player.Balance)
// 			game.IsActive = false
// 			bot.Send(tgbotapi.NewMessage(chatID, message))
// 			continue
// 		}

// 		message := fmt.Sprintf("💰 Ставка: %d\n💵 Баланс: %d\n\n🎴 Ваши карты: %v\nОчки: %d\n\n🃏 Карта дилера: %s", bet, player.Balance, game.PlayerCards, playerScore, game.DealerCards[0])

// 		keyboard := tgbotapi.NewInlineKeyboardMarkup(
// 			tgbotapi.NewInlineKeyboardRow(
// 				tgbotapi.NewInlineKeyboardButtonData("👊 Hit", "hit"),
// 				tgbotapi.NewInlineKeyboardButtonData("✋ Stand", "stand"),
// 			),
// 		)
// 		msg := tgbotapi.NewMessage(chatID, message)
// 		msg.ReplyMarkup = keyboard
// 		bot.Send(msg)
// 	}
// 	if strings.HasPrefix(text, "/balance") {
// 		player := getOrCreatePlayer(chatID)
// 		message := fmt.Sprintf("💰 Ваш баланс: %d\n🎮 Игр сыграно: %d\n✅ Побед: %d\n❌ Поражений: %d", player.Balance, player.Games, player.Wins, player.Losses)
// 		msg := tgbotapi.NewMessage(chatID, message)
// 		bot.Send(msg)
// 	}
// }

// func getRandomCard() string {
// 	if len(availableCards) == 0 {
// 		initDeck()
// 		fmt.Print("Дилер тасует карты")
// 	}

// 	card := availableCards[0]
// 	availableCards = availableCards[1:]
// 	return card
// }

// // Инициализируем колоду и приобвляем карты, что бы получилось 52 карты в колоде
// func initDeck() {
// 	availableCards = []string{}

// 	cards := []string{"2", "3", "4", "5", "6", "7", "8", "9", "10", "J", "Q", "K", "A"}

// 	for i := 0; i < 4; i++ {
// 		for _, card := range cards {
// 			availableCards = append(availableCards, card)
// 		}
// 	}

// 	rand.Shuffle(len(availableCards), func(i, j int) {
// 		availableCards[i], availableCards[j] = availableCards[j], availableCards[i]
// 	})
// }

// func init() {
// 	initDeck()
// }

// func winner(playerScore, dealerScore int) string {
// 	if playerScore > 21 {
// 		return "💥 Перебор! Вы проиграли!"
// 	} else if dealerScore > 21 {
// 		return "🎉 Дилер перебрал! Вы выиграли!"
// 	} else if playerScore > dealerScore {
// 		return "🎉 Вы выиграли!"
// 	} else if playerScore < dealerScore {
// 		return "😔 Дилер выиграл!"
// 	} else {
// 		return "🤝 Ничья!"
// 	}
// }

// func isBlackjack(cards []string) bool {
// 	if len(cards) != 2 {
// 		return false
// 	}
// 	score := calculateScore(cards)
// 	if score != 21 {
// 		return false
// 	}

// 	hasAce := false
// 	hasTen := false

// 	for _, card := range cards {
// 		if card == "A" {
// 			hasAce = true
// 		}
// 		if card == "10" || card == "J" || card == "Q" || card == "K" {
// 			hasTen = true
// 		}
// 	}
// 	return hasAce && hasTen
// }
