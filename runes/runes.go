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
// CardRole is the role of a rune in a composed spell.
type CardRole int

const (
	RoleCore     CardRole = iota // a spell on its own
	RoleModifier                 // attaches to a Core of matching Family
)

type World interface {
	// Damage / debuff: maxRange of 0 means unlimited; LOS is always required.
	DamageNearest(amount int, dt DamageType, maxRange float64)
	DamageAll(amount int, dt DamageType, maxRange float64)
	DelayNearest(turns int, maxRange float64)
	DelayAll(turns int)
	DrainNearest(amount int, maxRange float64)
	SacrificeNearestMinion(consumeHP, dmg int, maxRange float64)
	NearestHasIntentRune(maxRange float64) bool
	CopyNearestIntent(maxRange float64)

	// Targeting helper for CanPlay gating.
	HasTargetInRange(maxRange float64) bool

	GainArmor(amount int)
	GrantMovement(extra float64)
	HasMoved() bool
	ConsumeAllMovement()
	NearestIntendsAttack(maxRange float64) bool

	SummonMinion(power, hp int)
	SummonMinionAt(power, hp int, x, y float64)
	HasMinion() bool

	HealPlayer(amount int)
	LoseHP(amount int)
	AddEnergy(amount int)
	PlaceWall(cx, cy float64, length float64, hp int)

	// Composition / log surface for runes.
	HasModifier(name string) bool
	LogNote(text string)
}

// requireTargetInRange is a CanPlay helper for cards that target the nearest
// in-range enemy: blocks the play if no living enemy is reachable within the
// given world-distance plus line-of-sight.
func requireTargetInRange(r float64) func(World) (bool, string) {
	return func(w World) (bool, string) {
		if !w.HasTargetInRange(r) {
			return false, "no target in range"
		}
		return true, ""
	}
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
	// Range is the offensive range for hover preview and CanPlay gating.
	// 0 means "no range concept" (self-buff, summon, utility).
	Range float64
	// Slow runes make any spell containing them resolve AFTER the enemy turn.
	// They tend to be powerful effects whose downside is conceding initiative.
	Slow bool
	// CanPlay returns whether the card may be played right now and, if not,
	// a short reason for the UI. nil means always playable (subject to energy).
	CanPlay func(World) (bool, string)

	// --- Composition (spell grammar) ---

	// Role: Core (a spell on its own) or Modifier (attaches to a Core).
	Role CardRole
	// Family groups runes that accept the same modifiers. Two Cores in a spell
	// is forbidden regardless of Family; Family is used by modifier matching.
	Family string
	// ModifiesFamily: for Modifier-role cards, the Family of Core they attach to.
	ModifiesFamily string
}

func FireAttack() Card {
	const r = 200
	return Card{
		Name:        "Fire Attack",
		Glyph:       "ᚾ",
		Cost:        1,
		Range:       r,
		Description: "Deal 6 fire damage to the nearest enemy in range.",
		Effect:      func(w World) { w.DamageNearest(6, Fire, r) },
		CanPlay:     requireTargetInRange(r),
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
	const r = 200
	return Card{
		Name:        "Frost Attack",
		Glyph:       "ᛁ",
		Cost:        1,
		Range:       r,
		Description: "Deal 5 frost damage to the nearest enemy in range.",
		Effect:      func(w World) { w.DamageNearest(5, Frost, r) },
		CanPlay:     requireTargetInRange(r),
	}
}

func Move() Card {
	return Card{
		Name:        "Move",
		Glyph:       "ᚱ",
		Cost:        0,
		Description: "Gain 100 extra movement this turn.",
		Effect: func(w World) {
			w.GrantMovement(100)
		},
	}
}

// --- Mesmer cards ---

func AphyrDelay() Card {
	const r = 250
	return Card{
		Name:        "Aphyr",
		Glyph:       "ᚬ",
		Cost:        1,
		Range:       r,
		Description: "Delay the nearest enemy's next action by 1 turn. Slow.",
		Slow:        true,
		Effect:      func(w World) { w.DelayNearest(1, r) },
		CanPlay:     requireTargetInRange(r),
	}
}

// Isa deals damage when the nearest enemy's intent matches the spell's
// expectation. By default the spell expects the enemy to be attacking; the
// Inverse modifier flips that to non-attacking. On mismatch the rune fizzles.
func Isa() Card {
	const r = 200
	return Card{
		Name:        "Isa",
		Glyph:       "ᛁ",
		Cost:        1,
		Range:       r,
		Role:        RoleCore,
		Family:      "Isa",
		Description: "Deal 8 damage to the nearest enemy intending to attack. Fizzles otherwise. Inverse flips the condition.",
		Effect: func(w World) {
			intends := w.NearestIntendsAttack(r)
			expects := !w.HasModifier("Inverse")
			if intends != expects {
				if expects {
					w.LogNote("Isa fizzles: target is not intending to attack")
				} else {
					w.LogNote("Isa fizzles: target is intending to attack")
				}
				return
			}
			w.DamageNearest(8, Physical, r)
		},
		CanPlay: requireTargetInRange(r),
	}
}

// Inverse is a modifier rune that attaches to a Core of the "Isa" family,
// flipping its attacker / non-attacker condition. Useless on its own.
func Inverse() Card {
	return Card{
		Name:           "Inverse",
		Glyph:          "¬",
		Cost:           1,
		Role:           RoleModifier,
		ModifiesFamily: "Isa",
		Description:    "Modifier (Isa): flips the target condition. Useless on its own.",
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
	const r = 150
	return Card{
		Name:        "Maðr",
		Glyph:       "ᛘ",
		Cost:        1,
		Range:       r,
		Description: "Drain: deal 4 damage to the nearest enemy in range and heal 4.",
		Effect:      func(w World) { w.DrainNearest(4, r) },
		CanPlay:     requireTargetInRange(r),
	}
}

func Ar() Card {
	const r = 150
	return Card{
		Name:        "Ár",
		Glyph:       "ᛅ",
		Cost:        0,
		Range:       r,
		Description: "Sacrifice 4 HP from your nearest minion to deal 4 damage to the nearest enemy in range.",
		Effect:      func(w World) { w.SacrificeNearestMinion(4, 4, r) },
		CanPlay: func(w World) (bool, string) {
			if !w.HasMinion() {
				return false, "requires a living minion"
			}
			if !w.HasTargetInRange(r) {
				return false, "no target in range"
			}
			return true, ""
		},
	}
}

// --- Reward pool (cards offered between combats) ---

func StrongFireAttack() Card {
	const r = 200
	return Card{
		Name:        "Strong Fire Attack",
		Glyph:       "ᚾ+",
		Cost:        1,
		Range:       r,
		Description: "Deal 9 fire damage to the nearest enemy in range.",
		Effect:      func(w World) { w.DamageNearest(9, Fire, r) },
		CanPlay:     requireTargetInRange(r),
	}
}

func GreaterFireAttack() Card {
	const r = 250
	return Card{
		Name:        "Greater Fire Attack",
		Glyph:       "ᚾ++",
		Cost:        2,
		Range:       r,
		Description: "Deal 14 fire damage to the nearest enemy in range. Slow.",
		Slow:        true,
		Effect:      func(w World) { w.DamageNearest(14, Fire, r) },
		CanPlay:     requireTargetInRange(r),
	}
}

func Firestorm() Card {
	const r = 250
	return Card{
		Name:        "Firestorm",
		Glyph:       "ᚠ",
		Cost:        2,
		Range:       r,
		Description: "Deal 4 fire damage to every enemy in range with line-of-sight.",
		Effect:      func(w World) { w.DamageAll(4, Fire, r) },
		CanPlay:     requireTargetInRange(r),
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
	const r = 250
	return Card{
		Name:        "Greater Aphyr",
		Glyph:       "ᚬ+",
		Cost:        2,
		Range:       r,
		Description: "Delay the nearest enemy's next 2 actions.",
		Effect:      func(w World) { w.DelayNearest(2, r) },
		CanPlay:     requireTargetInRange(r),
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

// SharpIsa is the strong Isa variant. Same intent fizzle, same Inverse
// modifier compatibility — just bigger damage.
func SharpIsa() Card {
	const r = 200
	return Card{
		Name:        "Sharp Isa",
		Glyph:       "ᛁ+",
		Cost:        1,
		Range:       r,
		Role:        RoleCore,
		Family:      "Isa",
		Description: "Deal 13 damage to the nearest enemy intending to attack. Fizzles otherwise. Inverse flips the condition.",
		Effect: func(w World) {
			intends := w.NearestIntendsAttack(r)
			expects := !w.HasModifier("Inverse")
			if intends != expects {
				if expects {
					w.LogNote("Sharp Isa fizzles: target is not intending to attack")
				} else {
					w.LogNote("Sharp Isa fizzles: target is intending to attack")
				}
				return
			}
			w.DamageNearest(13, Physical, r)
		},
		CanPlay: requireTargetInRange(r),
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
	const r = 250
	return Card{
		Name:        "Mimic",
		Glyph:       "↻",
		Cost:        1,
		Range:       r,
		Description: "Copy the nearest enemy's intended rune into your hand.",
		Effect:      func(w World) { w.CopyNearestIntent(r) },
		CanPlay: func(w World) (bool, string) {
			if !w.NearestHasIntentRune(r) {
				return false, "nearest has no rune to copy"
			}
			return true, ""
		},
	}
}

func MassDrain() Card {
	const r = 200
	return Card{
		Name:        "Mass Drain",
		Glyph:       "ᛘ+",
		Cost:        2,
		Range:       r,
		Description: "Drain: deal 6 damage to the nearest enemy in range and heal 6.",
		Effect:      func(w World) { w.DrainNearest(6, r) },
		CanPlay:     requireTargetInRange(r),
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
			Sprint(),
			Mimic(),
			Inverse(),
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
	for i := 0; i < 6; i++ {
		deck = append(deck, Isa())
	}
	for i := 0; i < 2; i++ {
		deck = append(deck, Inverse())
	}
	for i := 0; i < 2; i++ {
		deck = append(deck, Move())
	}
	return deck
}
