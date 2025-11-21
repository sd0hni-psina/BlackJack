package game

func CalculateScore(hand []string) int {
	score := 0
	aces := 0

	for _, card := range hand {
		score += CardValues[card]
		if card == "A" {
			aces++
		}
	}

	for score > 21 && aces > 0 {
		score -= 10
		aces--
	}

	return score
}

func IsBlackjack(cards []string) bool {
	if len(cards) != 2 {
		return false
	}

	if CalculateScore(cards) != 21 {
		return false
	}

	hasAce, hasTen := false, false
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

func IsBust(cards []string) bool {
	return CalculateScore(cards) > 21
}
