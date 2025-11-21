package bot

import (
	"log"

	"blackjack/internal/config"
	"blackjack/internal/player"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api     *tgbotapi.BotAPI
	handler *Handler
}

func New(cfg *config.Config, repo player.Repository) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(cfg.BotToken)
	if err != nil {
		return nil, err
	}

	return &Bot{
		api:     api,
		handler: NewHandler(api, cfg, repo),
	}, nil
}

func (b *Bot) Run() error {
	log.Printf("Bot started: @%s", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			go b.handler.HandleCallback(update.CallbackQuery)
			continue
		}

		if update.Message != nil {
			go b.handler.HandleMessage(update.Message)
		}
	}

	return nil
}
