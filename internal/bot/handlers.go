package bot

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"blackjack/internal/config"
	"blackjack/internal/game"
	"blackjack/internal/player"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Handler struct {
	bot     *tgbotapi.BotAPI
	cfg     *config.Config
	players player.Repository
	games   *game.Manager
}

func NewHandler(bot *tgbotapi.BotAPI, cfg *config.Config, repo player.Repository) *Handler {
	return &Handler{
		bot:     bot,
		cfg:     cfg,
		players: repo,
		games:   game.NewManager(),
	}
}

// ============== ВСПОМОГАТЕЛЬНЫЕ МЕТОДЫ ==============

func (h *Handler) send(chatID int64, text string) {
	if _, err := h.bot.Send(tgbotapi.NewMessage(chatID, text)); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

func (h *Handler) sendWithKeyboard(chatID int64, text string, kb tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ReplyMarkup = kb
	if _, err := h.bot.Send(msg); err != nil {
		log.Printf("Failed to send message: %v", err)
	}
}

func (h *Handler) answerCallback(id, text string) {
	h.bot.Request(tgbotapi.NewCallback(id, text))
}

func (h *Handler) getPlayer(chatID int64) (*player.Player, error) {
	return h.players.GetOrCreate(chatID, h.cfg.StartBalance, h.cfg.DefaultBet)
}

func (h *Handler) savePlayer(p *player.Player) {
	if err := h.players.Save(p); err != nil {
		log.Printf("Failed to save player: %v", err)
	}
}

// ============== ФОРМАТИРОВАНИЕ ==============

func formatGameStatus(g *game.State, showDealerHand bool) string {
	dealerDisplay := fmt.Sprintf("[%s, ?]", g.DealerCards[0])
	if showDealerHand {
		dealerDisplay = fmt.Sprintf("%v (%d)", g.DealerCards, g.DealerScore())
	}

	return fmt.Sprintf("🎴 Вы: %v (%d)\n🃏 Дилер: %s",
		g.PlayerCards, g.PlayerScore(), dealerDisplay)
}

func formatGameEnd(g *game.State, p *player.Player, result string, winAmount int) string {
	msg := fmt.Sprintf("🎴 Вы: %v (%d)\n🃏 Дилер: %v (%d)\n\n%s",
		g.PlayerCards, g.PlayerScore(), g.DealerCards, g.DealerScore(), result)

	if winAmount > 0 {
		msg += fmt.Sprintf("\n💰 Выигрыш: +%d", winAmount)
	}
	msg += fmt.Sprintf("\n💵 Баланс: %d", p.Balance)

	return msg
}

// ============== ОБРАБОТЧИКИ КОМАНД ==============

func (h *Handler) HandleStart(chatID int64) {
	p, err := h.getPlayer(chatID)
	if err != nil {
		h.send(chatID, "❌ Ошибка. Попробуйте позже.")
		return
	}

	h.send(chatID, fmt.Sprintf(
		"🎰 Добро пожаловать в Blackjack!\n\n"+
			"💵 Баланс: %d\n\n"+
			"/play <ставка> — играть\n"+
			"/balance — статистика\n"+
			"/top — топ игроков\n"+
			"/help — правила",
		p.Balance))
}

func (h *Handler) HandleHelp(chatID int64) {
	h.send(chatID,
		"📖 Правила Blackjack:\n\n"+
			"🎯 Цель: набрать 21 очко или больше дилера, не перебрав\n\n"+
			"📊 Очки:\n"+
			"• 2-10 — номинал\n"+
			"• J, Q, K — 10\n"+
			"• A — 11 или 1\n\n"+
			"🎮 Действия:\n"+
			"• Hit — взять карту\n"+
			"• Stand — остановиться\n"+
			"• Double — удвоить (только первый ход)\n\n"+
			"🎰 Blackjack платит x2.5")
}

func (h *Handler) HandleBalance(chatID int64) {
	p, err := h.getPlayer(chatID)
	if err != nil {
		h.send(chatID, "❌ Ошибка")
		return
	}

	h.send(chatID, fmt.Sprintf(
		"💰 Баланс: %d\n\n"+
			"📊 Статистика:\n"+
			"🎮 Игр: %d\n"+
			"✅ Побед: %d (%.1f%%)\n"+
			"❌ Поражений: %d\n"+
			"🤝 Ничьих: %d",
		p.Balance, p.Games, p.Wins, p.WinRate(), p.Losses, p.Draws))
}

func (h *Handler) HandleTop(chatID int64) {
	stats, err := h.players.GetTopByBalance(10)
	if err != nil {
		h.send(chatID, "❌ Ошибка")
		return
	}

	if len(stats) == 0 {
		h.send(chatID, "🏆 Пока никто не играл!")
		return
	}

	var sb strings.Builder
	sb.WriteString("🏆 Топ игроков:\n\n")

	medals := []string{"🥇", "🥈", "🥉"}
	for i, s := range stats {
		medal := fmt.Sprintf("%d.", i+1)
		if i < 3 {
			medal = medals[i]
		}
		sb.WriteString(fmt.Sprintf("%s %d 💰 | %d игр (%.0f%%)\n",
			medal, s.Balance, s.Games, s.WinRate))
	}

	h.send(chatID, sb.String())
}

func (h *Handler) HandlePlay(chatID int64, args []string) {
	p, err := h.getPlayer(chatID)
	if err != nil {
		h.send(chatID, "❌ Ошибка")
		return
	}

	bet := h.cfg.DefaultBet
	if len(args) > 0 {
		if b, err := strconv.Atoi(args[0]); err == nil && b > 0 {
			bet = b
		} else {
			h.send(chatID, fmt.Sprintf("❌ Неверная ставка. Пример: /play %d", h.cfg.DefaultBet))
			return
		}
	}

	if bet < h.cfg.MinBet || bet > h.cfg.MaxBet {
		h.send(chatID, fmt.Sprintf("❌ Ставка от %d до %d", h.cfg.MinBet, h.cfg.MaxBet))
		return
	}

	if !p.PlaceBet(bet) {
		h.send(chatID, fmt.Sprintf("❌ Недостаточно средств! Баланс: %d", p.Balance))
		return
	}

	g := game.NewState(bet)
	h.games.Set(chatID, g)

	// Проверка блэкджеков
	playerBJ := game.IsBlackjack(g.PlayerCards)
	dealerBJ := game.IsBlackjack(g.DealerCards)

	if playerBJ || dealerBJ {
		g.IsActive = false

		if playerBJ && dealerBJ {
			p.AddDraw(bet)
			h.savePlayer(p)
			h.sendWithKeyboard(chatID,
				fmt.Sprintf("🎴 Вы: %v — BLACKJACK!\n🃏 Дилер: %v — BLACKJACK!\n\n🤝 Ничья!\n💵 Баланс: %d",
					g.PlayerCards, g.DealerCards, p.Balance),
				EndGameKeyboard(p.LastBet))
			return
		}

		if playerBJ {
			winAmount := int(float64(bet) * h.cfg.BlackjackPays)
			p.AddWin(winAmount)
			h.savePlayer(p)
			h.sendWithKeyboard(chatID,
				fmt.Sprintf("🎴 Вы: %v\n\n🎰 BLACKJACK! 🎰\n\n💰 +%d (x%.1f)\n💵 Баланс: %d",
					g.PlayerCards, winAmount, h.cfg.BlackjackPays, p.Balance),
				EndGameKeyboard(p.LastBet))
			return
		}

		// Дилер блэкджек
		p.AddLoss()
		h.savePlayer(p)
		h.sendWithKeyboard(chatID,
			fmt.Sprintf("🎴 Вы: %v (%d)\n🃏 Дилер: %v\n\n🎰 BLACKJACK у дилера!\n💵 Баланс: %d",
				g.PlayerCards, g.PlayerScore(), g.DealerCards, p.Balance),
			EndGameKeyboard(p.LastBet))
		return
	}

	h.savePlayer(p)
	canDouble := p.CanAfford(bet)

	h.sendWithKeyboard(chatID,
		fmt.Sprintf("💰 Ставка: %d | Баланс: %d\n\n%s",
			bet, p.Balance, formatGameStatus(g, false)),
		GameKeyboard(canDouble))
}

// ============== ОБРАБОТЧИКИ CALLBACK ==============

func (h *Handler) HandleCallback(callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	data := callback.Data

	p, err := h.getPlayer(chatID)
	if err != nil {
		h.answerCallback(callback.ID, "Ошибка")
		return
	}

	switch data {
	case CallbackPlayAgain:
		h.answerCallback(callback.ID, "")
		h.HandlePlay(chatID, []string{strconv.Itoa(p.LastBet)})
		return

	case CallbackBalance:
		h.answerCallback(callback.ID, fmt.Sprintf("💵 %d", p.Balance))
		return
	}

	g := h.games.Get(chatID)
	if g == nil || !g.IsActive {
		h.answerCallback(callback.ID, "Игра не активна")
		return
	}

	switch data {
	case CallbackHit:
		h.handleHit(chatID, g, p)
	case CallbackStand:
		h.handleStand(chatID, g, p)
	case CallbackDouble:
		h.handleDouble(chatID, g, p)
	}

	h.answerCallback(callback.ID, "")
}

func (h *Handler) handleHit(chatID int64, g *game.State, p *player.Player) {
	g.Hit()

	if game.IsBust(g.PlayerCards) {
		g.IsActive = false
		p.AddLoss()
		h.savePlayer(p)

		h.sendWithKeyboard(chatID,
			fmt.Sprintf("🎴 Вы: %v (%d)\n\n💥 Перебор!\n💵 Баланс: %d",
				g.PlayerCards, g.PlayerScore(), p.Balance),
			EndGameKeyboard(p.LastBet))
		return
	}

	h.sendWithKeyboard(chatID, formatGameStatus(g, false), GameKeyboard(false))
}

func (h *Handler) handleStand(chatID int64, g *game.State, p *player.Player) {
	result := g.Finish()
	var resultText string
	var winAmount int

	switch result {
	case game.ResultPlayerWin:
		resultText = "🎉 Вы выиграли!"
		winAmount = g.Bet * 2
		p.AddWin(winAmount)
	case game.ResultDealerWin:
		resultText = "😔 Дилер выиграл!"
		p.AddLoss()
	case game.ResultPush:
		resultText = "🤝 Ничья!"
		p.AddDraw(g.Bet)
	}

	h.savePlayer(p)
	h.sendWithKeyboard(chatID,
		formatGameEnd(g, p, resultText, winAmount),
		EndGameKeyboard(p.LastBet))
}

func (h *Handler) handleDouble(chatID int64, g *game.State, p *player.Player) {
	if !g.CanDouble {
		return
	}

	if !p.CanAfford(g.Bet) {
		h.send(chatID, "❌ Недостаточно средств для удвоения")
		return
	}

	p.Balance -= g.Bet
	g.Double()

	if game.IsBust(g.PlayerCards) {
		g.IsActive = false
		p.AddLoss()
		h.savePlayer(p)

		h.sendWithKeyboard(chatID,
			fmt.Sprintf("💰 Удвоено: %d\n\n🎴 Вы: %v (%d)\n\n💥 Перебор!\n💵 Баланс: %d",
				g.Bet, g.PlayerCards, g.PlayerScore(), p.Balance),
			EndGameKeyboard(p.LastBet))
		return
	}

	result := g.Finish()
	var resultText string
	var winAmount int

	switch result {
	case game.ResultPlayerWin:
		resultText = "🎉 Вы выиграли!"
		winAmount = g.Bet * 2
		p.AddWin(winAmount)
	case game.ResultDealerWin:
		resultText = "😔 Дилер выиграл!"
		p.AddLoss()
	case game.ResultPush:
		resultText = "🤝 Ничья!"
		p.AddDraw(g.Bet)
	}

	h.savePlayer(p)
	h.sendWithKeyboard(chatID,
		fmt.Sprintf("💰 Удвоено: %d\n\n%s", g.Bet, formatGameEnd(g, p, resultText, winAmount)),
		EndGameKeyboard(p.LastBet))
}

// ============== ОБРАБОТЧИК СООБЩЕНИЙ ==============

func (h *Handler) HandleMessage(msg *tgbotapi.Message) {
	chatID := msg.Chat.ID
	text := msg.Text
	parts := strings.Fields(text)

	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]

	switch {
	case cmd == "/start":
		h.HandleStart(chatID)
	case cmd == "/help":
		h.HandleHelp(chatID)
	case cmd == "/play":
		h.HandlePlay(chatID, args)
	case cmd == "/balance":
		h.HandleBalance(chatID)
	case cmd == "/top":
		h.HandleTop(chatID)
	}
}
