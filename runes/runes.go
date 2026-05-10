package runes

type DamageType int

const (
	Fire DamageType = iota
	Frost
	Physical
)

func (t DamageType) String() string {
	switch t {
	case Fire:
		return "fire"
	case Frost:
		return "frost"
	case Physical:
		return "physical"
	default:
		return "?"
	}
}

type Class int

const (
	ClassElementalist Class = iota
	ClassMesmer
	ClassNecromancer
)

func (c Class) String() string {
	switch c {
	case ClassElementalist:
		return "Elementalist"
	case ClassMesmer:
		return "Mesmer"
	case ClassNecromancer:
		return "Necromancer"
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
	SummonMinion(power, hp int)
	SummonMinionAt(power, hp int, x, y float64)
	DrainNearest(amount int)
	SacrificeNearestMinion(consumeHP, dmg int)
	HasMinion() bool
	HealPlayer(amount int)
	LoseHP(amount int)
	AddEnergy(amount int)
	NearestHasIntentRune() bool
	CopyNearestIntent()
	PlaceWall(cx, cy float64, length float64, hp int)
}

// PlacementShape tells the UI what kind of placement preview to draw and what
// the click point represents.
type PlacementShape int

const (
	PlacementPoint PlacementShape = iota // a single dot at the cursor
	PlacementWall                        // a perpendicular wall through the cursor
)

type Card struct {
	Name        string
	Glyph       string
	Cost        int
	Description string
	// Effect is the instant effect; runs immediately on play.
	Effect func(World)
	// PlacementEffect is an alternative to Effect: the card requires the
	// player to choose a target position on the radar before resolving.
	// (x, y) are world coordinates.
	PlacementEffect func(World, float64, float64)
	// PlacementShape controls how the placement target is previewed.
	PlacementShape PlacementShape
	// Slow runes make any spell containing them resolve AFTER the enemy turn.
	// They tend to be powerful effects whose downside is conceding initiative.
	Slow bool
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
		Description: "Delay the nearest enemy's next action by 1 turn. Slow.",
		Slow:        true,
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

// --- Necromancer cards ---

func Thurisaz() Card {
	return Card{
		Name:            "Thurisaz",
		Glyph:           "ᚦ",
		Cost:            2,
		Description:     "Summon a minion at a chosen location: 3 damage / turn (8 HP).",
		PlacementEffect: func(w World, x, y float64) { w.SummonMinionAt(3, 8, x, y) },
	}
}

func Madr() Card {
	return Card{
		Name:        "Maðr",
		Glyph:       "ᛘ",
		Cost:        1,
		Description: "Drain: deal 4 damage to the nearest enemy and heal 4.",
		Effect:      func(w World) { w.DrainNearest(4) },
	}
}

func Ar() Card {
	return Card{
		Name:        "Ár",
		Glyph:       "ᛅ",
		Cost:        0,
		Description: "Sacrifice 4 HP from your nearest minion to deal 4 damage to the nearest enemy.",
		Effect:      func(w World) { w.SacrificeNearestMinion(4, 4) },
		CanPlay: func(w World) (bool, string) {
			if !w.HasMinion() {
				return false, "requires a living minion"
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
		Description: "Deal 14 fire damage to the nearest enemy. Slow.",
		Slow:        true,
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

func WallOfStone() Card {
	return Card{
		Name:           "Wall of Stone",
		Glyph:          "ᛏ‖",
		Cost:           2,
		Description:    "Raise a stone wall (length 100, 24 HP) at a chosen point. Blocks movement; enemies can break it.",
		PlacementShape: PlacementWall,
		PlacementEffect: func(w World, x, y float64) {
			w.PlaceWall(x, y, 100, 24)
		},
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

func GreaterThurisaz() Card {
	return Card{
		Name:            "Greater Thurisaz",
		Glyph:           "ᚦ+",
		Cost:            2,
		Description:     "Summon a stronger minion at a chosen location: 5 damage / turn (12 HP).",
		PlacementEffect: func(w World, x, y float64) { w.SummonMinionAt(5, 12, x, y) },
	}
}

func Reanimate() Card {
	return Card{
		Name:        "Reanimate",
		Glyph:       "ᚢ",
		Cost:        2,
		Description: "Summon two weak minions at a chosen location: 2 damage / turn each (4 HP).",
		PlacementEffect: func(w World, x, y float64) {
			w.SummonMinionAt(2, 4, x, y)
			w.SummonMinionAt(2, 4, x+15, y)
		},
	}
}

func Mimic() Card {
	return Card{
		Name:        "Mimic",
		Glyph:       "↻",
		Cost:        1,
		Description: "Copy the nearest enemy's intended rune into your hand.",
		Effect:      func(w World) { w.CopyNearestIntent() },
		CanPlay: func(w World) (bool, string) {
			if !w.NearestHasIntentRune() {
				return false, "nearest has no rune to copy"
			}
			return true, ""
		},
	}
}

func MassDrain() Card {
	return Card{
		Name:        "Mass Drain",
		Glyph:       "ᛘ+",
		Cost:        2,
		Description: "Drain: deal 6 damage to the nearest enemy and heal 6.",
		Effect:      func(w World) { w.DrainNearest(6) },
	}
}

func Ritual() Card {
	return Card{
		Name:        "Ritual",
		Glyph:       "ᛟ",
		Cost:        0,
		Description: "Lose 5 HP. Gain 2 energy this turn.",
		Effect: func(w World) {
			w.LoseHP(5)
			w.AddEnergy(2)
		},
	}
}

// StarterDeck returns the starting deck for the chosen class.
func StarterDeck(c Class) []Card {
	switch c {
	case ClassMesmer:
		return mesmerStarter()
	case ClassNecromancer:
		return necromancerStarter()
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
			AphyrDelay(),
			SharpIsa(),
			ColdIsa(),
			Sprint(),
			Mimic(),
		}
	case ClassNecromancer:
		return []Card{
			GreaterThurisaz(),
			Reanimate(),
			MassDrain(),
			Ritual(),
		}
	default:
		return []Card{
			StrongFireAttack(),
			GreaterFireAttack(),
			Firestorm(),
			StoneSkin(),
			WallOfStone(),
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

func necromancerStarter() []Card {
	deck := make([]Card, 0, 10)
	for i := 0; i < 4; i++ {
		deck = append(deck, Thurisaz())
	}
	for i := 0; i < 3; i++ {
		deck = append(deck, Madr())
	}
	for i := 0; i < 3; i++ {
		deck = append(deck, Ar())
	}
	return deck
}

func mesmerStarter() []Card {
	deck := make([]Card, 0, 10)
	for i := 0; i < 4; i++ {
		deck = append(deck, IsaAggressive())
	}
	for i := 0; i < 4; i++ {
		deck = append(deck, IsaPassive())
	}
	for i := 0; i < 2; i++ {
		deck = append(deck, Move())
	}
	return deck
}
