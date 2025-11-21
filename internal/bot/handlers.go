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

func formatHandStatus(hand *game.Hand, index int, total int) string {
	prefix := "🎴"
	if total > 1 {
		prefix = fmt.Sprintf("🎴 Рука %d:", index+1)
	}

	status := ""
	if hand.IsBust {
		status = " 💥"
	} else if hand.IsStand {
		status = " ✋"
	}

	return fmt.Sprintf("%s %v (%d)%s", prefix, hand.Cards, hand.Score(), status)
}

func (h *Handler) formatGameStatus(g *game.State, showDealer bool) string {
	var sb strings.Builder

	// Показываем все руки
	for i, hand := range g.Hands {
		if i == g.CurrentHand && !g.AllHandsComplete() {
			sb.WriteString("👉 ") // Текущая рука
		}
		sb.WriteString(formatHandStatus(hand, i, len(g.Hands)))
		sb.WriteString("\n")
	}

	// Дилер
	if showDealer {
		sb.WriteString(fmt.Sprintf("🃏 Дилер: %v (%d)", g.DealerCards, g.DealerScore()))
	} else {
		sb.WriteString(fmt.Sprintf("🃏 Дилер: [%s, ?]", g.DealerCards[0]))
	}

	return sb.String()
}

func (h *Handler) formatGameEnd(g *game.State, p *player.Player, results []string, totalWin int) string {
	var sb strings.Builder

	// Руки игрока с результатами
	for i, hand := range g.Hands {
		sb.WriteString(formatHandStatus(hand, i, len(g.Hands)))
		if i < len(results) {
			sb.WriteString(" — ")
			sb.WriteString(results[i])
		}
		sb.WriteString("\n")
	}

	sb.WriteString(fmt.Sprintf("🃏 Дилер: %v (%d)\n", g.DealerCards, g.DealerScore()))

	if totalWin > 0 {
		sb.WriteString(fmt.Sprintf("\n💰 Выигрыш: +%d", totalWin))
	}
	sb.WriteString(fmt.Sprintf("\n💵 Баланс: %d", p.Balance))

	return sb.String()
}

func (h *Handler) getKeyboardOptions(g *game.State, p *player.Player) GameKeyboardOptions {
	hand := g.Current()
	if hand == nil {
		return GameKeyboardOptions{}
	}

	return GameKeyboardOptions{
		CanDouble: hand.CanDouble() && p.CanAfford(hand.Bet),
		CanSplit:  hand.CanSplit() && p.CanAfford(hand.Bet) && len(g.Hands) < 4,
	}
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
			"🎯 Цель: набрать 21 или больше дилера\n\n"+
			"📊 Очки: 2-10 номинал, J/Q/K = 10, A = 11 или 1\n\n"+
			"🎮 Действия:\n"+
			"• Hit — взять карту\n"+
			"• Stand — остановиться\n"+
			"• Double — удвоить ставку\n"+
			"• Split — разделить пару\n\n"+
			"✂️ Split: при двух одинаковых картах можно разделить на две руки. Каждая рука играет отдельно.\n\n"+
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

	hand := g.Current()

	// Проверка блэкджеков
	playerBJ := hand.IsBlackjack()
	dealerBJ := game.IsBlackjack(g.DealerCards)

	if playerBJ || dealerBJ {
		g.IsActive = false

		if playerBJ && dealerBJ {
			p.AddDraw(bet)
			h.savePlayer(p)
			h.sendWithKeyboard(chatID,
				fmt.Sprintf("🎴 Вы: %v — BLACKJACK!\n🃏 Дилер: %v — BLACKJACK!\n\n🤝 Ничья!\n💵 Баланс: %d",
					hand.Cards, g.DealerCards, p.Balance),
				EndGameKeyboard(p.LastBet))
			return
		}

		if playerBJ {
			winAmount := int(float64(bet) * h.cfg.BlackjackPays)
			p.AddWin(winAmount)
			h.savePlayer(p)
			h.sendWithKeyboard(chatID,
				fmt.Sprintf("🎴 Вы: %v\n\n🎰 BLACKJACK! 🎰\n\n💰 +%d (x%.1f)\n💵 Баланс: %d",
					hand.Cards, winAmount, h.cfg.BlackjackPays, p.Balance),
				EndGameKeyboard(p.LastBet))
			return
		}

		p.AddLoss()
		h.savePlayer(p)
		h.sendWithKeyboard(chatID,
			fmt.Sprintf("🎴 Вы: %v (%d)\n🃏 Дилер: %v\n\n🎰 BLACKJACK у дилера!\n💵 Баланс: %d",
				hand.Cards, hand.Score(), g.DealerCards, p.Balance),
			EndGameKeyboard(p.LastBet))
		return
	}

	h.savePlayer(p)

	opts := h.getKeyboardOptions(g, p)
	h.sendWithKeyboard(chatID,
		fmt.Sprintf("💰 Ставка: %d | Баланс: %d\n\n%s",
			bet, p.Balance, h.formatGameStatus(g, false)),
		GameKeyboard(opts))
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
	case CallbackSplit:
		h.handleSplit(chatID, g, p)
	}

	h.answerCallback(callback.ID, "")
}

func (h *Handler) handleHit(chatID int64, g *game.State, p *player.Player) {
	g.Hit()
	hand := g.Current()

	if hand.IsBust {
		// Переход к следующей руке или завершение
		if g.NextHand() {
			// Есть ещё руки
			opts := h.getKeyboardOptions(g, p)
			h.sendWithKeyboard(chatID,
				fmt.Sprintf("💥 Перебор на руке %d!\n\n%s",
					g.CurrentHand, h.formatGameStatus(g, false)),
				GameKeyboard(opts))
		} else {
			// Все руки сыграны
			h.finishGame(chatID, g, p)
		}
		return
	}

	opts := h.getKeyboardOptions(g, p)
	h.sendWithKeyboard(chatID, h.formatGameStatus(g, false), GameKeyboard(opts))
}

func (h *Handler) handleStand(chatID int64, g *game.State, p *player.Player) {
	g.Stand()

	if g.NextHand() {
		// Переход к следующей руке
		opts := h.getKeyboardOptions(g, p)
		h.sendWithKeyboard(chatID,
			fmt.Sprintf("✋ Стоим. Переход к руке %d\n\n%s",
				g.CurrentHand+1, h.formatGameStatus(g, false)),
			GameKeyboard(opts))
	} else {
		h.finishGame(chatID, g, p)
	}
}

func (h *Handler) handleDouble(chatID int64, g *game.State, p *player.Player) {
	hand := g.Current()
	if hand == nil || !hand.CanDouble() {
		return
	}

	if !p.CanAfford(hand.Bet) {
		h.send(chatID, "❌ Недостаточно средств для удвоения")
		return
	}

	p.Balance -= hand.Bet
	g.Double()

	if g.NextHand() {
		status := "✋"
		if hand.IsBust {
			status = "💥"
		}
		opts := h.getKeyboardOptions(g, p)
		h.sendWithKeyboard(chatID,
			fmt.Sprintf("💰 Удвоено! %s Переход к руке %d\n\n%s",
				status, g.CurrentHand+1, h.formatGameStatus(g, false)),
			GameKeyboard(opts))
	} else {
		h.finishGame(chatID, g, p)
	}
}

func (h *Handler) handleSplit(chatID int64, g *game.State, p *player.Player) {
	hand := g.Current()
	if hand == nil || !hand.CanSplit() {
		return
	}

	if !p.CanAfford(hand.Bet) {
		h.send(chatID, "❌ Недостаточно средств для сплита")
		return
	}

	// Списываем ставку для новой руки
	p.Balance -= hand.Bet
	h.savePlayer(p)

	g.Split()

	// Если сплит тузов — сразу завершаем
	if hand.SplitAces {
		h.send(chatID, "✂️ Сплит тузов! По одной карте на каждую руку.")
		h.finishGame(chatID, g, p)
		return
	}

	opts := h.getKeyboardOptions(g, p)
	h.sendWithKeyboard(chatID,
		fmt.Sprintf("✂️ Сплит! Теперь у вас %d руки.\n💰 Общая ставка: %d | Баланс: %d\n\n%s",
			len(g.Hands), g.TotalBet(), p.Balance, h.formatGameStatus(g, false)),
		GameKeyboard(opts))
}

func (h *Handler) finishGame(chatID int64, g *game.State, p *player.Player) {
	g.Finish()

	var results []string
	totalWin := 0
	wins := 0
	losses := 0
	draws := 0

	for _, hand := range g.Hands {
		result, winAmount := g.HandResult(hand)

		switch result {
		case game.ResultPlayerWin:
			results = append(results, "🎉 Победа!")
			totalWin += winAmount
			wins++
		case game.ResultDealerWin:
			results = append(results, "😔 Проигрыш")
			losses++
		case game.ResultPush:
			results = append(results, "🤝 Ничья")
			totalWin += winAmount
			draws++
		}
	}

	// Обновляем баланс и статистику
	p.Balance += totalWin

	// Считаем как одну игру, но записываем все победы/поражения
	if wins > losses {
		p.Wins++
	} else if losses > wins {
		p.Losses++
	} else {
		p.Draws++
	}
	p.Games++

	h.savePlayer(p)

	h.sendWithKeyboard(chatID,
		h.formatGameEnd(g, p, results, totalWin),
		EndGameKeyboard(g.InitialBet))
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
