package runes

type DamageType int

const (
	Fire DamageType = iota
	Frost
	Physical
)

type Class int

const (
	ClassElementalist Class = iota
	ClassMesmer
)

func (c Class) String() string {
	switch c {
	case ClassElementalist:
		return "Elementalist"
	case ClassMesmer:
		return "Mesmer"
	default:
		return "Unknown"
	}
}

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
	NearestIntendsAttack() bool
	DelayNearest(turns int)
	DelayAll(turns int)
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

// --- Mesmer cards ---

func AphyrDelay() Card {
	return Card{
		Name:        "Aphyr",
		Glyph:       "ᚬ",
		Cost:        1,
		Description: "Delay the nearest enemy's next action by 1 turn.",
		Effect:      func(w World) { w.DelayNearest(1) },
	}
}

func IsaAggressive() Card {
	return Card{
		Name:        "Isa (aggressive)",
		Glyph:       "ᛁ↯",
		Cost:        1,
		Description: "Deal 8 damage. Castable only if the nearest enemy intends to attack.",
		Effect:      func(w World) { w.DamageNearest(8, Physical) },
		CanPlay: func(w World) (bool, string) {
			if !w.NearestIntendsAttack() {
				return false, "nearest must intend to attack"
			}
			return true, ""
		},
	}
}

func IsaPassive() Card {
	return Card{
		Name:        "Isa (passive)",
		Glyph:       "ᛁ◦",
		Cost:        1,
		Description: "Deal 8 damage. Castable only if the nearest enemy does NOT intend to attack.",
		Effect:      func(w World) { w.DamageNearest(8, Physical) },
		CanPlay: func(w World) (bool, string) {
			if w.NearestIntendsAttack() {
				return false, "nearest must not intend to attack"
			}
			return true, ""
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

func GreaterAphyr() Card {
	return Card{
		Name:        "Greater Aphyr",
		Glyph:       "ᚬ+",
		Cost:        2,
		Description: "Delay the nearest enemy's next 2 actions.",
		Effect:      func(w World) { w.DelayNearest(2) },
	}
}

func MassDelay() Card {
	return Card{
		Name:        "Mass Delay",
		Glyph:       "ᚬ*",
		Cost:        2,
		Description: "Delay every enemy's next action by 1 turn.",
		Effect:      func(w World) { w.DelayAll(1) },
	}
}

func SharpIsa() Card {
	return Card{
		Name:        "Sharp Isa",
		Glyph:       "ᛁ↯+",
		Cost:        1,
		Description: "Deal 13 damage. Castable only if the nearest enemy intends to attack.",
		Effect:      func(w World) { w.DamageNearest(13, Physical) },
		CanPlay: func(w World) (bool, string) {
			if !w.NearestIntendsAttack() {
				return false, "nearest must intend to attack"
			}
			return true, ""
		},
	}
}

func ColdIsa() Card {
	return Card{
		Name:        "Cold Isa",
		Glyph:       "ᛁ◦+",
		Cost:        1,
		Description: "Deal 13 damage. Castable only if the nearest enemy does NOT intend to attack.",
		Effect:      func(w World) { w.DamageNearest(13, Physical) },
		CanPlay: func(w World) (bool, string) {
			if w.NearestIntendsAttack() {
				return false, "nearest must not intend to attack"
			}
			return true, ""
		},
	}
}

// StarterDeck returns the starting deck for the chosen class.
func StarterDeck(c Class) []Card {
	switch c {
	case ClassMesmer:
		return mesmerStarter()
	default:
		return elementalistStarter()
	}
}

// RewardPool returns the reward cards offered between combats for the chosen
// class.
func RewardPool(c Class) []Card {
	switch c {
	case ClassMesmer:
		return []Card{
			GreaterAphyr(),
			MassDelay(),
			SharpIsa(),
			ColdIsa(),
			Sprint(),
		}
	default:
		return []Card{
			StrongFireAttack(),
			GreaterFireAttack(),
			Firestorm(),
			StoneSkin(),
		}
	}
}

func elementalistStarter() []Card {
	deck := make([]Card, 0, 9)
	for i := 0; i < 4; i++ {
		deck = append(deck, FireAttack())
	}
	for i := 0; i < 3; i++ {
		deck = append(deck, EarthArmor())
	}
	for i := 0; i < 2; i++ {
		deck = append(deck, FrostAttack())
	}
	return deck
}

func mesmerStarter() []Card {
	deck := make([]Card, 0, 10)
	for i := 0; i < 2; i++ {
		deck = append(deck, AphyrDelay())
	}
	for i := 0; i < 3; i++ {
		deck = append(deck, IsaAggressive())
	}
	for i := 0; i < 3; i++ {
		deck = append(deck, IsaPassive())
	}
	for i := 0; i < 2; i++ {
		deck = append(deck, Move())
	}
	return deck
}
