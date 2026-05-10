package enemies

type Enemy struct {
	Name        string
	HP, MaxHP   int
	X, Y        float64 // radar position; player is at (0,0)
	MeleeRange  float64
	MoveSpeed   float64
	AttackPower int
	Intent      string // human-readable next action
}

func NewGoblin(x, y float64) *Enemy {
	return &Enemy{
		Name:        "Goblin",
		HP:          22, MaxHP: 22,
		X: x, Y: y,
		MeleeRange:  40,
		MoveSpeed:   60,
		AttackPower: 6,
	}
}

func NewWraith(x, y float64) *Enemy {
	return &Enemy{
		Name:        "Wraith",
		HP:          16, MaxHP: 16,
		X: x, Y: y,
		MeleeRange:  35,
		MoveSpeed:   45,
		AttackPower: 8,
	}
}
