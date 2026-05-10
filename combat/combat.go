package combat

import (
	"fmt"
	"math"
	"math/rand"

	"deckbuilder/enemies"
	"deckbuilder/runes"
)

const (
	StartingEnergy   = 3
	HandSize         = 5
	BaseMovement     = 30
	DefaultMaxHP     = 70
	EnemyTurnStepDur = 0.4 // seconds per enemy action
)

type Phase int

const (
	PhasePlayer Phase = iota
	PhaseEnemy
	PhaseWon
	PhaseLost
)

// DamagePopup is a short-lived floating number anchored in radar coordinates
// (player-relative). UI reads these directly.
type DamagePopup struct {
	X, Y   float64
	Amount int
	Type   runes.DamageType
	Age    float64 // seconds
}

const PopupLife = 1.1

// Minion is a Necromancer-summoned process that occupies a radar position and
// runs an embedded program every turn (currently: damage nearest enemy).
type Minion struct {
	HP, MaxHP   int
	X, Y        float64
	AttackPower int
}

type Combat struct {
	PlayerHP, PlayerMaxHP int
	PlayerArmor           int
	Energy, MaxEnergy     int

	Enemies []*enemies.Enemy
	Minions []*Minion

	Draw, Hand, Discard []runes.Card

	MovementBudget float64 // remaining movement this turn
	hasMoved       bool
	Phase          Phase

	Popups []DamagePopup

	// Enemy turn animation state
	enemyIndex   int
	enemyTimer  float64

	rng *rand.Rand
}

func New(seed int64, hp, maxHP int, deck []runes.Card, foes []*enemies.Enemy) *Combat {
	deckCopy := make([]runes.Card, len(deck))
	copy(deckCopy, deck)
	c := &Combat{
		PlayerHP:    hp,
		PlayerMaxHP: maxHP,
		Enemies:     foes,
		Draw:        deckCopy,
		Phase:       PhasePlayer,
		rng:         rand.New(rand.NewSource(seed)),
	}
	c.shuffle(c.Draw)
	c.startPlayerTurn()
	return c
}

func (c *Combat) shuffle(cards []runes.Card) {
	c.rng.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
}

func (c *Combat) startPlayerTurn() {
	c.compactMinions()
	c.runMinionPrograms()
	c.checkVictory()
	if c.Phase == PhaseWon {
		return
	}
	c.Energy = StartingEnergy
	c.MaxEnergy = StartingEnergy
	c.PlayerArmor = 0
	c.MovementBudget = BaseMovement
	c.hasMoved = false
	c.refreshIntents()
	c.drawUpTo(HandSize)
	c.Phase = PhasePlayer
}

func (c *Combat) compactMinions() {
	out := c.Minions[:0]
	for _, m := range c.Minions {
		if m.HP > 0 {
			out = append(out, m)
		}
	}
	c.Minions = out
}

func (c *Combat) runMinionPrograms() {
	for _, m := range c.Minions {
		if m.HP <= 0 {
			continue
		}
		var target *enemies.Enemy
		best := math.Inf(1)
		for _, e := range c.Enemies {
			if e.HP <= 0 {
				continue
			}
			d := math.Hypot(e.X-m.X, e.Y-m.Y)
			if d < best {
				best = d
				target = e
			}
		}
		if target == nil {
			continue
		}
		dealt := m.AttackPower
		if target.Weakness == runes.Physical {
			dealt = (dealt*3 + 1) / 2
		}
		target.HP -= dealt
		if target.HP < 0 {
			target.HP = 0
		}
		c.Popups = append(c.Popups, DamagePopup{
			X: target.X, Y: target.Y, Amount: dealt, Type: runes.Physical,
		})
	}
}

func (c *Combat) drawUpTo(n int) {
	for len(c.Hand) < n {
		if len(c.Draw) == 0 {
			if len(c.Discard) == 0 {
				return
			}
			c.Draw = append(c.Draw, c.Discard...)
			c.Discard = c.Discard[:0]
			c.shuffle(c.Draw)
		}
		c.Hand = append(c.Hand, c.Draw[0])
		c.Draw = c.Draw[1:]
	}
}

// PlayCard plays the card at hand index i. Returns true if played.
func (c *Combat) PlayCard(i int) bool {
	if c.Phase != PhasePlayer || i < 0 || i >= len(c.Hand) {
		return false
	}
	card := c.Hand[i]
	if card.Cost > c.Energy {
		return false
	}
	if card.CanPlay != nil {
		if ok, _ := card.CanPlay(c); !ok {
			return false
		}
	}
	c.Energy -= card.Cost
	card.Effect(c)
	c.Hand = append(c.Hand[:i], c.Hand[i+1:]...)
	c.Discard = append(c.Discard, card)
	c.refreshIntents()
	c.checkVictory()
	return true
}

// MoveTowards moves the player toward the given world target, consuming
// movement budget. Player is always at (0,0), so this just shifts all enemy
// positions by -delta. Returns the actual distance moved.
func (c *Combat) MoveTowards(tx, ty float64) float64 {
	if c.Phase != PhasePlayer || c.MovementBudget <= 0 {
		return 0
	}
	dist := math.Hypot(tx, ty)
	if dist == 0 {
		return 0
	}
	step := math.Min(dist, c.MovementBudget)
	dx := tx / dist * step
	dy := ty / dist * step
	for _, e := range c.Enemies {
		e.X -= dx
		e.Y -= dy
	}
	for _, m := range c.Minions {
		m.X -= dx
		m.Y -= dy
	}
	for i := range c.Popups {
		c.Popups[i].X -= dx
		c.Popups[i].Y -= dy
	}
	c.MovementBudget -= step
	c.hasMoved = true
	return step
}

func (c *Combat) EndTurn() {
	if c.Phase != PhasePlayer {
		return
	}
	c.Discard = append(c.Discard, c.Hand...)
	c.Hand = c.Hand[:0]
	c.Phase = PhaseEnemy
	c.enemyIndex = 0
	c.enemyTimer = 0
}

// Update advances the enemy phase animation. dt is seconds.
func (c *Combat) Update(dt float64) {
	c.advancePopups(dt)
	if c.Phase != PhaseEnemy {
		return
	}
	c.enemyTimer += dt
	if c.enemyTimer < EnemyTurnStepDur {
		return
	}
	c.enemyTimer = 0
	for c.enemyIndex < len(c.Enemies) {
		e := c.Enemies[c.enemyIndex]
		c.enemyIndex++
		if e.HP <= 0 {
			continue
		}
		if e.Stunned > 0 {
			e.Stunned--
			e.Intent = "delayed"
			continue
		}
		c.runEnemyProgram(e)
		if c.PlayerHP <= 0 {
			c.Phase = PhaseLost
			return
		}
		return // one enemy per tick
	}
	c.startPlayerTurn()
	c.checkVictory()
}

// aggroTarget represents what an enemy will hit this turn. Enemies aggro on
// the nearest dot — player or minion — implementing the design's "dumb enemies
// always target nearest" rule.
type aggroTarget struct {
	minion *Minion // nil means the player at (0,0)
	x, y   float64
}

func (t aggroTarget) isMinion() bool { return t.minion != nil }

func (c *Combat) chooseTarget(e *enemies.Enemy) aggroTarget {
	best := aggroTarget{x: 0, y: 0}
	bestDist := math.Hypot(e.X, e.Y)
	for _, m := range c.Minions {
		if m.HP <= 0 {
			continue
		}
		d := math.Hypot(e.X-m.X, e.Y-m.Y)
		if d < bestDist {
			bestDist = d
			best = aggroTarget{minion: m, x: m.X, y: m.Y}
		}
	}
	return best
}

// Default enemy program (design §8). Ranged casters drift forward while
// chipping every turn; pure melee enemies move-or-attack. Either may strike
// the player or an intercepting minion, whichever is nearest.
func (c *Combat) runEnemyProgram(e *enemies.Enemy) {
	t := c.chooseTarget(e)
	dist := math.Hypot(e.X-t.x, e.Y-t.y)
	apply := func(dmg int) {
		if t.isMinion() {
			t.minion.HP -= dmg
			if t.minion.HP < 0 {
				t.minion.HP = 0
			}
			c.Popups = append(c.Popups, DamagePopup{
				X: t.x, Y: t.y, Amount: dmg, Type: runes.Physical,
			})
			return
		}
		c.applyDamageToPlayer(dmg)
	}
	moveToward := func(step float64) {
		dx := t.x - e.X
		dy := t.y - e.Y
		n := math.Hypot(dx, dy)
		if n == 0 {
			return
		}
		e.X += dx / n * step
		e.Y += dy / n * step
	}

	if e.RangedPower > 0 {
		apply(e.RangedPower)
		e.Intent = "cast"
		if dist > e.MeleeRange {
			step := math.Min(dist-e.MeleeRange, e.MoveSpeed)
			if step > 0 {
				moveToward(step)
			}
		}
		return
	}
	if dist <= e.MeleeRange {
		apply(e.AttackPower)
		e.Intent = "attacked"
		return
	}
	step := math.Min(dist-e.MeleeRange, e.MoveSpeed)
	if step <= 0 {
		return
	}
	moveToward(step)
	e.Intent = "approached"
}

// refreshIntents previews each enemy's next action for UI display.
func (c *Combat) refreshIntents() {
	for _, e := range c.Enemies {
		if e.HP <= 0 {
			e.Intent = ""
			continue
		}
		if e.Stunned > 0 {
			e.Intent = "delayed"
			continue
		}
		t := c.chooseTarget(e)
		dist := math.Hypot(e.X-t.x, e.Y-t.y)
		suffix := ""
		if t.isMinion() {
			suffix = " minion"
		}
		switch {
		case e.RangedPower > 0:
			e.Intent = fmt.Sprintf("cast%s (%d)", suffix, e.RangedPower)
		case dist <= e.MeleeRange:
			e.Intent = fmt.Sprintf("attack%s (%d)", suffix, e.AttackPower)
		default:
			e.Intent = "approach" + suffix
		}
	}
}

// nearestLiving returns the nearest enemy with HP > 0, ignoring stunned status.
func (c *Combat) nearestLiving() *enemies.Enemy {
	var target *enemies.Enemy
	best := math.Inf(1)
	for _, e := range c.Enemies {
		if e.HP <= 0 {
			continue
		}
		d := math.Hypot(e.X, e.Y)
		if d < best {
			best = d
			target = e
		}
	}
	return target
}

func (c *Combat) applyDamageToPlayer(amount int) {
	if c.PlayerArmor >= amount {
		c.PlayerArmor -= amount
		return
	}
	amount -= c.PlayerArmor
	c.PlayerArmor = 0
	c.PlayerHP -= amount
	if c.PlayerHP < 0 {
		c.PlayerHP = 0
	}
}

func (c *Combat) checkVictory() {
	for _, e := range c.Enemies {
		if e.HP > 0 {
			return
		}
	}
	c.Phase = PhaseWon
}

// --- runes.World implementation ---

func (c *Combat) DamageNearest(amount int, dt runes.DamageType) {
	var target *enemies.Enemy
	best := math.Inf(1)
	for _, e := range c.Enemies {
		if e.HP <= 0 {
			continue
		}
		d := math.Hypot(e.X, e.Y)
		if d < best {
			best = d
			target = e
		}
	}
	if target == nil {
		return
	}
	dealt := amount
	if target.Weakness == dt {
		dealt = (amount*3 + 1) / 2 // 1.5x rounded
	}
	target.HP -= dealt
	if target.HP < 0 {
		target.HP = 0
	}
	c.Popups = append(c.Popups, DamagePopup{
		X: target.X, Y: target.Y, Amount: dealt, Type: dt,
	})
}

func (c *Combat) advancePopups(dt float64) {
	if len(c.Popups) == 0 {
		return
	}
	out := c.Popups[:0]
	for _, p := range c.Popups {
		p.Age += dt
		if p.Age < PopupLife {
			out = append(out, p)
		}
	}
	c.Popups = out
}

func (c *Combat) DamageAll(amount int, dt runes.DamageType) {
	for _, e := range c.Enemies {
		if e.HP <= 0 {
			continue
		}
		dealt := amount
		if e.Weakness == dt {
			dealt = (amount*3 + 1) / 2
		}
		e.HP -= dealt
		if e.HP < 0 {
			e.HP = 0
		}
		c.Popups = append(c.Popups, DamagePopup{
			X: e.X, Y: e.Y, Amount: dealt, Type: dt,
		})
	}
}

func (c *Combat) GainArmor(amount int) { c.PlayerArmor += amount }

func (c *Combat) GrantMovement(extra float64) { c.MovementBudget += extra }

func (c *Combat) HasMoved() bool { return c.hasMoved }

func (c *Combat) ConsumeAllMovement() { c.MovementBudget = 0 }

// NearestIntendsAttack returns true if the nearest living, non-stunned enemy
// will deal damage to the player on its next turn (not to a minion).
func (c *Combat) NearestIntendsAttack() bool {
	var target *enemies.Enemy
	best := math.Inf(1)
	for _, e := range c.Enemies {
		if e.HP <= 0 || e.Stunned > 0 {
			continue
		}
		d := math.Hypot(e.X, e.Y)
		if d < best {
			best = d
			target = e
		}
	}
	if target == nil {
		return false
	}
	t := c.chooseTarget(target)
	if t.isMinion() {
		return false
	}
	if target.RangedPower > 0 {
		return true
	}
	return math.Hypot(target.X, target.Y) <= target.MeleeRange
}

func (c *Combat) DelayNearest(turns int) {
	t := c.nearestLiving()
	if t == nil {
		return
	}
	t.Stunned += turns
}

func (c *Combat) DelayAll(turns int) {
	for _, e := range c.Enemies {
		if e.HP <= 0 {
			continue
		}
		e.Stunned += turns
	}
}

// --- Necromancer World methods ---

func (c *Combat) SummonMinion(power, hp int) {
	x, y := 50.0, 0.0
	if t := c.nearestLiving(); t != nil {
		d := math.Hypot(t.X, t.Y)
		if d > 0 {
			x = t.X / d * 50
			y = t.Y / d * 50
		}
	}
	c.Minions = append(c.Minions, &Minion{
		HP: hp, MaxHP: hp,
		X: x, Y: y,
		AttackPower: power,
	})
}

func (c *Combat) DrainNearest(amount int) {
	c.DamageNearest(amount, runes.Physical)
	c.HealPlayer(amount)
}

func (c *Combat) HealPlayer(amount int) {
	c.PlayerHP += amount
	if c.PlayerHP > c.PlayerMaxHP {
		c.PlayerHP = c.PlayerMaxHP
	}
}

func (c *Combat) SacrificeNearestMinion(consumeHP, dmg int) {
	var nearest *Minion
	best := math.Inf(1)
	for _, m := range c.Minions {
		if m.HP <= 0 {
			continue
		}
		d := math.Hypot(m.X, m.Y)
		if d < best {
			best = d
			nearest = m
		}
	}
	if nearest == nil {
		return
	}
	nearest.HP -= consumeHP
	if nearest.HP < 0 {
		nearest.HP = 0
	}
	c.DamageNearest(dmg, runes.Physical)
}

func (c *Combat) HasMinion() bool {
	for _, m := range c.Minions {
		if m.HP > 0 {
			return true
		}
	}
	return false
}

func (c *Combat) LoseHP(amount int) {
	c.PlayerHP -= amount
	if c.PlayerHP < 0 {
		c.PlayerHP = 0
	}
	if c.PlayerHP == 0 {
		c.Phase = PhaseLost
	}
}

func (c *Combat) AddEnergy(amount int) { c.Energy += amount }
