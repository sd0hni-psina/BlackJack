package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	CallbackHit       = "hit"
	CallbackStand     = "stand"
	CallbackDouble    = "double"
	CallbackSplit     = "split"
	CallbackPlayAgain = "play_again"
	CallbackBalance   = "balance"
)

type GameKeyboardOptions struct {
	CanDouble bool
	CanSplit  bool
}

func GameKeyboard(opts GameKeyboardOptions) tgbotapi.InlineKeyboardMarkup {
	row := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("👊 Hit", CallbackHit),
		tgbotapi.NewInlineKeyboardButtonData("✋ Stand", CallbackStand),
	}

	if opts.CanDouble {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("💰 Double", CallbackDouble))
	}
	if opts.CanSplit {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("✂️ Split", CallbackSplit))
	}

	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func EndGameKeyboard(lastBet int) tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData(
				fmt.Sprintf("🔄 Ещё (%d)", lastBet),
				CallbackPlayAgain,
			),
			tgbotapi.NewInlineKeyboardButtonData("💵 Баланс", CallbackBalance),
		),
	)
}
