package runes

type DamageType int

const (
	Fire DamageType = iota
	Frost
	Physical
)

// Effect applies a card's effect to the combat world. The combat package
// implements World; runes only depend on this small interface to avoid an
// import cycle.
type World interface {
	DamageNearest(amount int, dt DamageType)
	GainArmor(amount int)
	GrantMovement(extra float64)
}

type Card struct {
	Name   string
	Glyph  string
	Cost   int
	Effect func(World)
}

func NAUD() Card {
	return Card{
		Name:  "Naud",
		Glyph: "ᚾ",
		Cost:  1,
		Effect: func(w World) {
			w.DamageNearest(6, Fire)
		},
	}
}

func Truth() Card {
	return Card{
		Name:  "Truth",
		Glyph: "ᛦ",
		Cost:  1,
		Effect: func(w World) {
			w.GainArmor(8)
		},
	}
}

func Isa() Card {
	return Card{
		Name:  "Isa",
		Glyph: "ᛁ",
		Cost:  1,
		Effect: func(w World) {
			w.DamageNearest(5, Frost)
		},
	}
}

func Raido() Card {
	return Card{
		Name:  "Raido",
		Glyph: "ᚱ",
		Cost:  0,
		Effect: func(w World) {
			w.GrantMovement(80)
		},
	}
}

// ElementalistStarter returns the 10-card Elementalist starting deck.
func ElementalistStarter() []Card {
	deck := make([]Card, 0, 10)
	for i := 0; i < 4; i++ {
		deck = append(deck, NAUD())
	}
	for i := 0; i < 3; i++ {
		deck = append(deck, Truth())
	}
	for i := 0; i < 2; i++ {
		deck = append(deck, Isa())
	}
	deck = append(deck, Raido())
	return deck
}
