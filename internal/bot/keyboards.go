package bot

import (
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	CallbackHit       = "hit"
	CallbackStand     = "stand"
	CallbackDouble    = "double"
	CallbackPlayAgain = "play_again"
	CallbackBalance   = "balance"
)

func GameKeyboard(canDouble bool) tgbotapi.InlineKeyboardMarkup {
	row := []tgbotapi.InlineKeyboardButton{
		tgbotapi.NewInlineKeyboardButtonData("👊 Hit", CallbackHit),
		tgbotapi.NewInlineKeyboardButtonData("✋ Stand", CallbackStand),
	}

	if canDouble {
		row = append(row, tgbotapi.NewInlineKeyboardButtonData("💰 Double", CallbackDouble))
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
