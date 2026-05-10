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
	DamageAll(amount int, dt DamageType)
	GainArmor(amount int)
	GrantMovement(extra float64)
	HasMoved() bool
	ConsumeAllMovement()
}

type Card struct {
	Name        string
	Glyph       string
	Cost        int
	Description string
	Effect      func(World)
	// CanPlay returns whether the card may be played right now and, if not,
	// a short reason for the UI. nil means always playable (subject to energy).
	CanPlay func(World) (bool, string)
}

func FireAttack() Card {
	return Card{
		Name:        "Fire Attack",
		Glyph:       "ᚾ",
		Cost:        1,
		Description: "Deal 6 fire damage to the nearest enemy.",
		Effect: func(w World) {
			w.DamageNearest(6, Fire)
		},
	}
}

func EarthArmor() Card {
	return Card{
		Name:        "Earth Armor",
		Glyph:       "ᛦ",
		Cost:        1,
		Description: "Gain 8 armor. Only castable if you have not moved; ends your movement.",
		Effect: func(w World) {
			w.GainArmor(8)
			w.ConsumeAllMovement()
		},
		CanPlay: func(w World) (bool, string) {
			if w.HasMoved() {
				return false, "requires no movement this turn"
			}
			return true, ""
		},
	}
}

func FrostAttack() Card {
	return Card{
		Name:        "Frost Attack",
		Glyph:       "ᛁ",
		Cost:        1,
		Description: "Deal 5 frost damage to the nearest enemy.",
		Effect: func(w World) {
			w.DamageNearest(5, Frost)
		},
	}
}

func Move() Card {
	return Card{
		Name:        "Move",
		Glyph:       "ᚱ",
		Cost:        0,
		Description: "Gain 80 extra movement this turn.",
		Effect: func(w World) {
			w.GrantMovement(80)
		},
	}
}

// --- Reward pool (cards offered between combats) ---

func StrongFireAttack() Card {
	return Card{
		Name:        "Strong Fire Attack",
		Glyph:       "ᚾ+",
		Cost:        1,
		Description: "Deal 9 fire damage to the nearest enemy.",
		Effect:      func(w World) { w.DamageNearest(9, Fire) },
	}
}

func GreaterFireAttack() Card {
	return Card{
		Name:        "Greater Fire Attack",
		Glyph:       "ᚾ++",
		Cost:        2,
		Description: "Deal 14 fire damage to the nearest enemy.",
		Effect:      func(w World) { w.DamageNearest(14, Fire) },
	}
}

func Firestorm() Card {
	return Card{
		Name:        "Firestorm",
		Glyph:       "ᚠ",
		Cost:        2,
		Description: "Deal 4 fire damage to all enemies.",
		Effect:      func(w World) { w.DamageAll(4, Fire) },
	}
}

func StoneSkin() Card {
	return Card{
		Name:        "Stone Skin",
		Glyph:       "ᛏ",
		Cost:        2,
		Description: "Gain 12 armor.",
		Effect:      func(w World) { w.GainArmor(12) },
	}
}

func Sprint() Card {
	return Card{
		Name:        "Sprint",
		Glyph:       "ᚱ+",
		Cost:        1,
		Description: "Gain 150 extra movement this turn.",
		Effect:      func(w World) { w.GrantMovement(150) },
	}
}

// RewardPool returns all cards the run may offer between combats.
func RewardPool() []Card {
	return []Card{
		StrongFireAttack(),
		GreaterFireAttack(),
		Firestorm(),
		StoneSkin(),
		Sprint(),
	}
}

// ElementalistStarter returns the 10-card Elementalist starting deck.
func ElementalistStarter() []Card {
	deck := make([]Card, 0, 10)
	for i := 0; i < 4; i++ {
		deck = append(deck, FireAttack())
	}
	for i := 0; i < 3; i++ {
		deck = append(deck, EarthArmor())
	}
	for i := 0; i < 2; i++ {
		deck = append(deck, FrostAttack())
	}
	deck = append(deck, Move())
	return deck
}
