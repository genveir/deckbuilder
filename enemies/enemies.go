package enemies

import "deckbuilder/runes"

type Enemy struct {
	Name        string
	HP, MaxHP   int
	X, Y        float64 // radar position; player is at (0,0)
	MeleeRange  float64
	MoveSpeed   float64
	AttackPower int
	RangedPower int              // 0 = no ranged attack
	RangedType  runes.DamageType // damage type of ranged attack (Physical default)
	MaxRange    float64          // max distance for the ranged attack; 0 = unlimited
	Stunned     int              // turns remaining where the enemy skips its action
	Weakness    runes.DamageType // hidden from the player
	Intent      string           // human-readable next action
}

func NewGoblin(x, y float64) *Enemy {
	return &Enemy{
		Name:        "Goblin",
		HP:          22, MaxHP: 22,
		X: x, Y: y,
		MeleeRange:  40,
		MoveSpeed:   100,
		AttackPower: 6,
		Weakness:    runes.Frost,
	}
}

func NewWraith(x, y float64) *Enemy {
	return &Enemy{
		Name:        "Wraith",
		HP:          16, MaxHP: 16,
		X: x, Y: y,
		MeleeRange:  35,
		MoveSpeed:   25,
		AttackPower: 0,
		RangedPower: 5,
		RangedType:  runes.Frost,
		MaxRange:    300,
		Weakness:    runes.Fire,
	}
}

func NewTroll(x, y float64) *Enemy {
	return &Enemy{
		Name:        "Troll",
		HP:          55, MaxHP: 55,
		X: x, Y: y,
		MeleeRange:  45,
		MoveSpeed:   30,
		AttackPower: 13,
		Weakness:    runes.Fire,
	}
}
