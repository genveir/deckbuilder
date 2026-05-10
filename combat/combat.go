package combat

import (
	"fmt"
	"math"
	"math/rand"
	"strings"

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

// DamagePopup is a short-lived floating number anchored in world coordinates.
// UI translates to radar via the player's current position.
type DamagePopup struct {
	X, Y   float64
	Amount int
	Type   runes.DamageType
	Age    float64 // seconds
}

const PopupLife = 1.1

// Minion is a Necromancer-summoned process that occupies a world position and
// runs an embedded program every turn (currently: damage nearest enemy).
type Minion struct {
	HP, MaxHP   int
	X, Y        float64
	AttackPower int
}

// Wall is an impassable line segment in world coordinates with HP. Enemies
// can break through by attacking it.
type Wall struct {
	X1, Y1   float64
	X2, Y2   float64
	HP, MaxHP int
}

// Player tracks the player's position in world coordinates. The radar is
// rendered relative to this position.
type Player struct {
	X, Y float64
}

// StagedCard is a rune queued for a single composed spell, paired with its
// placement target if the rune required one.
type StagedCard struct {
	Card           runes.Card
	PlaceX, PlaceY float64
}

// LogKind classifies log entries so the UI can colour them.
type LogKind int

const (
	LogPlayer LogKind = iota
	LogEnemy
	LogMinion
	LogSystem
)

// LogEntry is one line in the combat event log.
type LogEntry struct {
	Text string
	Kind LogKind
}

const MaxLogEntries = 80

type Combat struct {
	Player                Player
	PlayerHP, PlayerMaxHP int
	PlayerArmor           int
	Energy, MaxEnergy     int

	Enemies []*enemies.Enemy
	Minions []*Minion
	Walls   []*Wall

	Draw, Hand, Discard []runes.Card

	// Stage holds runes queued for the current turn's single composed spell.
	// SpellCast is true once the spell has been cast — no more staging this turn.
	Stage     []StagedCard
	SpellCast bool

	// pendingSlowSpell holds a slow-cast spell awaiting resolution after the
	// enemy phase. Set when the player casts a spell containing any Slow rune.
	pendingSlowSpell []StagedCard

	MovementBudget float64 // remaining movement this turn
	hasMoved       bool
	Phase          Phase

	Turn int // player turns elapsed in this combat (1-based)

	Log   []LogEntry
	actor string // name of the entity currently producing log entries

	Popups []DamagePopup

	// PendingCardIdx is the hand index of a card awaiting placement target.
	// -1 when no card is pending. While >= 0, other input is blocked.
	PendingCardIdx int

	// Enemy turn animation state
	enemyIndex int
	enemyTimer float64

	grid *navGrid

	rng *rand.Rand
}

func New(seed int64, hp, maxHP int, deck []runes.Card, foes []*enemies.Enemy) *Combat {
	deckCopy := make([]runes.Card, len(deck))
	copy(deckCopy, deck)
	c := &Combat{
		PlayerHP:       hp,
		PlayerMaxHP:    maxHP,
		Enemies:        foes,
		Draw:           deckCopy,
		Phase:          PhasePlayer,
		PendingCardIdx: -1,
		rng:            rand.New(rand.NewSource(seed)),
	}
	c.shuffle(c.Draw)
	c.startPlayerTurn()
	return c
}

func (c *Combat) addLog(kind LogKind, format string, args ...interface{}) {
	entry := LogEntry{Kind: kind, Text: fmt.Sprintf(format, args...)}
	c.Log = append(c.Log, entry)
	if len(c.Log) > MaxLogEntries {
		c.Log = c.Log[len(c.Log)-MaxLogEntries:]
	}
}

func (c *Combat) shuffle(cards []runes.Card) {
	c.rng.Shuffle(len(cards), func(i, j int) { cards[i], cards[j] = cards[j], cards[i] })
}

func (c *Combat) startPlayerTurn() {
	c.compactMinions()
	c.compactWalls()
	c.actor = "Minion"
	c.runMinionPrograms()
	c.actor = ""
	c.checkVictory()
	if c.Phase == PhaseWon {
		c.addLog(LogSystem, "All enemies defeated")
		return
	}
	c.Turn++
	c.addLog(LogSystem, "— Turn %d —", c.Turn)
	c.Energy = StartingEnergy
	c.MaxEnergy = StartingEnergy
	c.PlayerArmor = 0
	c.MovementBudget = BaseMovement
	c.hasMoved = false
	c.Stage = c.Stage[:0]
	c.SpellCast = false
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
		if target.HP == 0 {
			c.addLog(LogMinion, "Minion slays %s (%d dmg)", target.Name, dealt)
		} else {
			c.addLog(LogMinion, "Minion deals %d to %s (%d/%d)", dealt, target.Name, target.HP, target.MaxHP)
		}
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

// StageCard adds a rune to the spell being composed this turn. Only one
// spell may be cast per turn, so once SpellCast is true, no more staging.
// Placement-required cards return (true, true) and stay pending until
// ConfirmPlacement; instant-effect cards stage immediately.
func (c *Combat) StageCard(i int) (staged, needsPlacement bool) {
	if c.Phase != PhasePlayer || c.SpellCast || c.PendingCardIdx >= 0 {
		return false, false
	}
	if i < 0 || i >= len(c.Hand) {
		return false, false
	}
	card := c.Hand[i]
	if card.Cost > c.Energy {
		return false, false
	}
	if card.CanPlay != nil {
		if ok, _ := card.CanPlay(c); !ok {
			return false, false
		}
	}
	if card.PlacementEffect != nil {
		c.PendingCardIdx = i
		return true, true
	}
	c.Energy -= card.Cost
	c.Stage = append(c.Stage, StagedCard{Card: card})
	c.Hand = append(c.Hand[:i], c.Hand[i+1:]...)
	return true, false
}

// ConfirmPlacement stages the pending card with the chosen world target.
func (c *Combat) ConfirmPlacement(x, y float64) bool {
	if c.PendingCardIdx < 0 || c.PendingCardIdx >= len(c.Hand) {
		c.PendingCardIdx = -1
		return false
	}
	i := c.PendingCardIdx
	card := c.Hand[i]
	c.Energy -= card.Cost
	c.Stage = append(c.Stage, StagedCard{Card: card, PlaceX: x, PlaceY: y})
	c.Hand = append(c.Hand[:i], c.Hand[i+1:]...)
	c.PendingCardIdx = -1
	return true
}

func (c *Combat) CancelPlacement() { c.PendingCardIdx = -1 }

// UnstageLast returns the most recently staged rune to the player's hand,
// refunding its energy. Used as a one-step undo before the spell is cast.
func (c *Combat) UnstageLast() bool {
	if c.SpellCast || len(c.Stage) == 0 {
		return false
	}
	last := c.Stage[len(c.Stage)-1]
	c.Stage = c.Stage[:len(c.Stage)-1]
	c.Energy += last.Card.Cost
	c.Hand = append(c.Hand, last.Card)
	return true
}

// StageIsSlow reports whether any rune currently staged is Slow — meaning
// casting will defer resolution until after the enemy phase.
func (c *Combat) StageIsSlow() bool {
	for _, sc := range c.Stage {
		if sc.Card.Slow {
			return true
		}
	}
	return false
}

// CastSpell resolves all staged runes. If any are Slow, the spell is queued
// to resolve after the upcoming enemy phase and the player turn ends
// immediately. Movement budget is unaffected by casting itself.
func (c *Combat) CastSpell() bool {
	if c.Phase != PhasePlayer || c.SpellCast || len(c.Stage) == 0 {
		return false
	}
	names := make([]string, 0, len(c.Stage))
	for _, sc := range c.Stage {
		names = append(names, sc.Card.Name)
	}
	verb := "Cast"
	if c.StageIsSlow() {
		verb = "Slow cast (after enemy turn)"
	}
	c.addLog(LogPlayer, "%s: %s", verb, strings.Join(names, " + "))

	if c.StageIsSlow() {
		c.pendingSlowSpell = append(c.pendingSlowSpell[:0], c.Stage...)
		c.Stage = c.Stage[:0]
		c.SpellCast = true
		c.EndTurn() // hand to discard, transition to PhaseEnemy
		return true
	}
	c.actor = "Player"
	c.resolveSpell(c.Stage)
	c.actor = ""
	c.Stage = c.Stage[:0]
	c.SpellCast = true
	c.refreshIntents()
	c.checkVictory()
	return true
}

// resolveSpell runs the effects of every staged rune in order and discards
// the cards. Caller is responsible for clearing the source slice.
func (c *Combat) resolveSpell(stage []StagedCard) {
	for _, sc := range stage {
		switch {
		case sc.Card.Effect != nil:
			sc.Card.Effect(c)
		case sc.Card.PlacementEffect != nil:
			sc.Card.PlacementEffect(c, sc.PlaceX, sc.PlaceY)
		}
	}
	for _, sc := range stage {
		c.Discard = append(c.Discard, sc.Card)
	}
}

// MoveTowards moves the player along the given offset (radar-relative delta),
// consuming movement budget. Walls block movement: the player stops just
// before a wall they would otherwise cross. Returns the distance moved.
func (c *Combat) MoveTowards(dx, dy float64) float64 {
	if c.Phase != PhasePlayer || c.MovementBudget <= 0 {
		return 0
	}
	dist := math.Hypot(dx, dy)
	if dist == 0 {
		return 0
	}
	step := math.Min(dist, c.MovementBudget)
	tx := c.Player.X + dx/dist*step
	ty := c.Player.Y + dy/dist*step
	if hit, hx, hy := c.firstWallHit(c.Player.X, c.Player.Y, tx, ty); hit != nil {
		// Stop just shy of the intersection.
		ndx := hx - c.Player.X
		ndy := hy - c.Player.Y
		nd := math.Hypot(ndx, ndy)
		if nd <= 1.5 {
			return 0
		}
		k := (nd - 1.5) / nd
		tx = c.Player.X + ndx*k
		ty = c.Player.Y + ndy*k
		step = nd - 1.5
	}
	c.Player.X = tx
	c.Player.Y = ty
	c.MovementBudget -= step
	if step > 0 {
		c.hasMoved = true
	}
	return step
}

// firstWallHit returns the wall hit by segment (x1,y1)->(x2,y2) closest to
// (x1,y1), or nil if none.
func (c *Combat) firstWallHit(x1, y1, x2, y2 float64) (*Wall, float64, float64) {
	var best *Wall
	bestT := math.Inf(1)
	var bx, by float64
	for _, w := range c.Walls {
		if w.HP <= 0 {
			continue
		}
		ok, t, hx, hy := segIntersect(x1, y1, x2, y2, w.X1, w.Y1, w.X2, w.Y2)
		if ok && t < bestT {
			bestT = t
			bx, by = hx, hy
			best = w
		}
	}
	return best, bx, by
}

// segIntersect returns whether two segments intersect, the parameter t along
// the first segment (0..1), and the intersection point.
func segIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) (bool, float64, float64, float64) {
	rX := bx - ax
	rY := by - ay
	sX := dx - cx
	sY := dy - cy
	denom := rX*sY - rY*sX
	if denom == 0 {
		return false, 0, 0, 0
	}
	t := ((cx-ax)*sY - (cy-ay)*sX) / denom
	u := ((cx-ax)*rY - (cy-ay)*rX) / denom
	if t < 0 || t > 1 || u < 0 || u > 1 {
		return false, 0, 0, 0
	}
	return true, t, ax + t*rX, ay + t*rY
}

// closestPointOnSegment returns the point on segment (X1,Y1)-(X2,Y2) closest
// to (px, py).
func closestPointOnSegment(px, py, x1, y1, x2, y2 float64) (float64, float64) {
	dx := x2 - x1
	dy := y2 - y1
	len2 := dx*dx + dy*dy
	if len2 == 0 {
		return x1, y1
	}
	t := ((px-x1)*dx + (py-y1)*dy) / len2
	if t < 0 {
		t = 0
	} else if t > 1 {
		t = 1
	}
	return x1 + t*dx, y1 + t*dy
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
			c.addLog(LogEnemy, "%s is delayed", e.Name)
			continue
		}
		c.runEnemyProgram(e)
		if c.PlayerHP <= 0 {
			c.Phase = PhaseLost
			return
		}
		return // one enemy per tick
	}
	// All enemies have acted. Resolve any deferred slow spell before starting
	// the next player turn.
	if len(c.pendingSlowSpell) > 0 {
		c.addLog(LogPlayer, "Slow spell resolves")
		c.actor = "Player"
		c.resolveSpell(c.pendingSlowSpell)
		c.actor = ""
		c.pendingSlowSpell = c.pendingSlowSpell[:0]
		c.checkVictory()
		if c.Phase == PhaseWon {
			return
		}
	}
	c.startPlayerTurn()
	c.checkVictory()
}

// aggroTarget represents what an enemy will hit this turn. Enemies aggro on
// the nearest dot — player or minion — implementing the design's "dumb enemies
// always target nearest" rule.
type aggroTarget struct {
	minion *Minion // nil means the player
	x, y   float64
}

func (t aggroTarget) isMinion() bool { return t.minion != nil }

func (c *Combat) chooseTarget(e *enemies.Enemy) aggroTarget {
	best := aggroTarget{x: c.Player.X, y: c.Player.Y}
	bestDist := math.Hypot(e.X-c.Player.X, e.Y-c.Player.Y)
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
//
// Pathing: melee enemies use grid A* to route around walls when possible.
// If no path exists (target enclosed) or the enemy is already adjacent to a
// blocker on the direct line, they smash the wall instead of detouring.
// Ranged spells ignore walls.
func (c *Combat) runEnemyProgram(e *enemies.Enemy) {
	t := c.chooseTarget(e)
	dist := math.Hypot(e.X-t.x, e.Y-t.y)
	apply := func(dmg int, dt runes.DamageType) {
		if t.isMinion() {
			t.minion.HP -= dmg
			if t.minion.HP < 0 {
				t.minion.HP = 0
			}
			c.Popups = append(c.Popups, DamagePopup{
				X: t.x, Y: t.y, Amount: dmg, Type: dt,
			})
			if t.minion.HP == 0 {
				c.addLog(LogEnemy, "%s destroys a minion (%d %s)", e.Name, dmg, dt)
			} else {
				c.addLog(LogEnemy, "%s hits minion for %d %s (%d/%d)", e.Name, dmg, dt, t.minion.HP, t.minion.MaxHP)
			}
			return
		}
		c.applyDamageToPlayer(dmg)
		c.addLog(LogEnemy, "%s hits Player for %d %s (HP %d/%d, armor %d)", e.Name, dmg, dt, c.PlayerHP, c.PlayerMaxHP, c.PlayerArmor)
	}
	moveTowardPoint := func(tx, ty, step float64) {
		dx := tx - e.X
		dy := ty - e.Y
		n := math.Hypot(dx, dy)
		if n == 0 {
			return
		}
		s := math.Min(step, n)
		e.X += dx / n * s
		e.Y += dy / n * s
	}

	if e.RangedPower > 0 {
		inRange := e.MaxRange == 0 || dist <= e.MaxRange
		canCast := inRange && c.hasLOS(e.X, e.Y, t.x, t.y)
		if canCast {
			apply(e.RangedPower, e.RangedType)
			e.Intent = "cast"
		} else {
			e.Intent = "advance"
		}
		// Drift forward (path-aware) until in melee range.
		if dist > e.MeleeRange {
			if path := c.findPath(e.X, e.Y, t.x, t.y); len(path) >= 2 {
				nx, ny := c.smoothPathStep(e.X, e.Y, path, e.MoveSpeed)
				moveTowardPoint(nx, ny, e.MoveSpeed)
			}
		}
		return
	}

	// Melee: can we hit the target right now? Direct LOS + range required.
	if dist <= e.MeleeRange && c.hasLOS(e.X, e.Y, t.x, t.y) {
		apply(e.AttackPower, runes.Physical)
		e.Intent = "attacked"
		return
	}

	// Try to path around walls.
	if path := c.findPath(e.X, e.Y, t.x, t.y); len(path) >= 2 {
		nx, ny := c.smoothPathStep(e.X, e.Y, path, e.MoveSpeed)
		moveTowardPoint(nx, ny, e.MoveSpeed)
		e.Intent = "approached"
		return
	}

	// No path — fall back to wall-smashing on the direct line.
	if blocker, _, _ := c.firstWallHit(e.X, e.Y, t.x, t.y); blocker != nil {
		cpx, cpy := closestPointOnSegment(e.X, e.Y, blocker.X1, blocker.Y1, blocker.X2, blocker.Y2)
		distToWall := math.Hypot(cpx-e.X, cpy-e.Y)
		if distToWall <= e.MeleeRange {
			blocker.HP -= e.AttackPower
			if blocker.HP < 0 {
				blocker.HP = 0
			}
			e.Intent = fmt.Sprintf("smashing wall (%d)", e.AttackPower)
			c.Popups = append(c.Popups, DamagePopup{
				X: cpx, Y: cpy, Amount: e.AttackPower, Type: runes.Physical,
			})
			if blocker.HP == 0 {
				c.addLog(LogEnemy, "%s shatters a wall", e.Name)
			} else {
				c.addLog(LogEnemy, "%s strikes wall for %d (%d/%d)", e.Name, e.AttackPower, blocker.HP, blocker.MaxHP)
			}
			return
		}
		step := math.Min(distToWall-e.MeleeRange, e.MoveSpeed)
		if step > 0 {
			moveTowardPoint(cpx, cpy, step)
			e.Intent = "approaching wall"
			return
		}
	}
	e.Intent = "blocked"
}

// hasLOS reports whether a straight segment from (x1,y1) to (x2,y2) crosses
// any standing wall. Used as a melee-line-of-sight check.
func (c *Combat) hasLOS(x1, y1, x2, y2 float64) bool {
	w, _, _ := c.firstWallHit(x1, y1, x2, y2)
	return w == nil
}

// smoothPathStep walks the path from (sx,sy) and returns the world target the
// enemy should head toward this turn — the farthest waypoint within both
// line-of-sight and a generous step lookahead. Produces straighter motion
// than naively chasing the next waypoint.
func (c *Combat) smoothPathStep(sx, sy float64, path []cellPos, step float64) (float64, float64) {
	// Skip the source cell if present.
	startIdx := 0
	if len(path) > 0 {
		px, py := cellToWorld(path[0].X, path[0].Y)
		if math.Hypot(sx-px, sy-py) < cellSize {
			startIdx = 1
		}
	}
	if startIdx >= len(path) {
		return sx, sy
	}
	bestX, bestY := cellToWorld(path[startIdx].X, path[startIdx].Y)
	for i := startIdx + 1; i < len(path); i++ {
		wx, wy := cellToWorld(path[i].X, path[i].Y)
		if !c.hasLOS(sx, sy, wx, wy) {
			break
		}
		bestX, bestY = wx, wy
		// Don't bother looking past one step worth of distance.
		if math.Hypot(wx-sx, wy-sy) > step*1.5 {
			break
		}
	}
	return bestX, bestY
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
		if e.RangedPower > 0 {
			inRange := e.MaxRange == 0 || dist <= e.MaxRange
			canCast := inRange && c.hasLOS(e.X, e.Y, t.x, t.y)
			if canCast {
				e.Intent = fmt.Sprintf("cast %s%s (%d)", e.RangedType, suffix, e.RangedPower)
			} else {
				e.Intent = "advance"
			}
			continue
		}
		if dist <= e.MeleeRange && c.hasLOS(e.X, e.Y, t.x, t.y) {
			e.Intent = fmt.Sprintf("attack%s (%d)", suffix, e.AttackPower)
			continue
		}
		if path := c.findPath(e.X, e.Y, t.x, t.y); len(path) >= 2 {
			e.Intent = "approach" + suffix
			continue
		}
		if blocker, _, _ := c.firstWallHit(e.X, e.Y, t.x, t.y); blocker != nil {
			cpx, cpy := closestPointOnSegment(e.X, e.Y, blocker.X1, blocker.Y1, blocker.X2, blocker.Y2)
			if math.Hypot(cpx-e.X, cpy-e.Y) <= e.MeleeRange {
				e.Intent = fmt.Sprintf("smash wall (%d)", e.AttackPower)
			} else {
				e.Intent = "approach wall"
			}
			continue
		}
		e.Intent = "blocked"
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
		d := math.Hypot(e.X-c.Player.X, e.Y-c.Player.Y)
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

// nearestEnemyInRange returns the nearest living enemy that is within
// maxRange of the player and has line-of-sight. maxRange of 0 means
// unlimited.
func (c *Combat) nearestEnemyInRange(maxRange float64) *enemies.Enemy {
	var target *enemies.Enemy
	best := math.Inf(1)
	for _, e := range c.Enemies {
		if e.HP <= 0 {
			continue
		}
		d := math.Hypot(e.X-c.Player.X, e.Y-c.Player.Y)
		if maxRange > 0 && d > maxRange {
			continue
		}
		if !c.hasLOS(c.Player.X, c.Player.Y, e.X, e.Y) {
			continue
		}
		if d < best {
			best = d
			target = e
		}
	}
	return target
}

func (c *Combat) HasTargetInRange(maxRange float64) bool {
	return c.nearestEnemyInRange(maxRange) != nil
}

func (c *Combat) DamageNearest(amount int, dt runes.DamageType, maxRange float64) {
	target := c.nearestEnemyInRange(maxRange)
	if target == nil {
		c.addLog(LogPlayer, "%s spell: no target in range", c.actorOr("Spell"))
		return
	}
	dealt := amount
	weak := target.Weakness == dt
	if weak {
		dealt = (amount*3 + 1) / 2 // 1.5x rounded
	}
	target.HP -= dealt
	if target.HP < 0 {
		target.HP = 0
	}
	c.Popups = append(c.Popups, DamagePopup{
		X: target.X, Y: target.Y, Amount: dealt, Type: dt,
	})
	weakNote := ""
	if weak {
		weakNote = " (weak)"
	}
	if target.HP == 0 {
		c.addLog(LogPlayer, "%s slays %s with %d %s%s", c.actorOr("Player"), target.Name, dealt, dt, weakNote)
	} else {
		c.addLog(LogPlayer, "%s deals %d %s to %s%s (%d/%d)", c.actorOr("Player"), dealt, dt, target.Name, weakNote, target.HP, target.MaxHP)
	}
}

func (c *Combat) actorOr(fallback string) string {
	if c.actor == "" {
		return fallback
	}
	return c.actor
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

func (c *Combat) DamageAll(amount int, dt runes.DamageType, maxRange float64) {
	hits := 0
	for _, e := range c.Enemies {
		if e.HP <= 0 {
			continue
		}
		if maxRange > 0 {
			d := math.Hypot(e.X-c.Player.X, e.Y-c.Player.Y)
			if d > maxRange {
				continue
			}
		}
		if !c.hasLOS(c.Player.X, c.Player.Y, e.X, e.Y) {
			continue
		}
		dealt := amount
		weak := e.Weakness == dt
		if weak {
			dealt = (amount*3 + 1) / 2
		}
		e.HP -= dealt
		if e.HP < 0 {
			e.HP = 0
		}
		c.Popups = append(c.Popups, DamagePopup{
			X: e.X, Y: e.Y, Amount: dealt, Type: dt,
		})
		hits++
		weakNote := ""
		if weak {
			weakNote = " (weak)"
		}
		if e.HP == 0 {
			c.addLog(LogPlayer, "AOE slays %s with %d %s%s", e.Name, dealt, dt, weakNote)
		} else {
			c.addLog(LogPlayer, "AOE deals %d %s to %s%s (%d/%d)", dealt, dt, e.Name, weakNote, e.HP, e.MaxHP)
		}
	}
	if hits == 0 {
		c.addLog(LogPlayer, "AOE finds no targets")
	}
}

func (c *Combat) GainArmor(amount int) {
	c.PlayerArmor += amount
	c.addLog(LogPlayer, "Gain %d armor (now %d)", amount, c.PlayerArmor)
}

func (c *Combat) GrantMovement(extra float64) {
	c.MovementBudget += extra
	c.addLog(LogPlayer, "+%.0f movement (now %.0f)", extra, c.MovementBudget)
}

func (c *Combat) HasMoved() bool { return c.hasMoved }

func (c *Combat) ConsumeAllMovement() {
	if c.MovementBudget > 0 {
		c.addLog(LogPlayer, "Movement locked")
	}
	c.MovementBudget = 0
}

// NearestIntendsAttack returns true if the nearest living, non-stunned enemy
// will deal damage to the player on its next turn (not to a minion).
func (c *Combat) NearestIntendsAttack() bool {
	target := c.nearestNonStunned()
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
	return math.Hypot(target.X-c.Player.X, target.Y-c.Player.Y) <= target.MeleeRange
}

func (c *Combat) DelayNearest(turns int, maxRange float64) {
	t := c.nearestEnemyInRange(maxRange)
	if t == nil {
		c.addLog(LogPlayer, "Delay finds no target")
		return
	}
	t.Stunned += turns
	c.addLog(LogPlayer, "Delays %s for %d turn(s)", t.Name, turns)
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
	x, y := c.Player.X+50, c.Player.Y
	if t := c.nearestLiving(); t != nil {
		ndx := t.X - c.Player.X
		ndy := t.Y - c.Player.Y
		d := math.Hypot(ndx, ndy)
		if d > 0 {
			x = c.Player.X + ndx/d*50
			y = c.Player.Y + ndy/d*50
		}
	}
	c.SummonMinionAt(power, hp, x, y)
}

func (c *Combat) SummonMinionAt(power, hp int, x, y float64) {
	c.Minions = append(c.Minions, &Minion{
		HP: hp, MaxHP: hp,
		X: x, Y: y,
		AttackPower: power,
	})
	c.addLog(LogPlayer, "Summon minion (%d/turn, %d HP)", power, hp)
}

func (c *Combat) DrainNearest(amount int, maxRange float64) {
	target := c.nearestEnemyInRange(maxRange)
	if target == nil {
		return
	}
	c.DamageNearest(amount, runes.Physical, maxRange)
	c.HealPlayer(amount)
}

func (c *Combat) HealPlayer(amount int) {
	before := c.PlayerHP
	c.PlayerHP += amount
	if c.PlayerHP > c.PlayerMaxHP {
		c.PlayerHP = c.PlayerMaxHP
	}
	c.addLog(LogPlayer, "Heal %d (HP %d → %d)", c.PlayerHP-before, before, c.PlayerHP)
}

func (c *Combat) SacrificeNearestMinion(consumeHP, dmg int, maxRange float64) {
	var nearest *Minion
	best := math.Inf(1)
	for _, m := range c.Minions {
		if m.HP <= 0 {
			continue
		}
		d := math.Hypot(m.X-c.Player.X, m.Y-c.Player.Y)
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
	if nearest.HP == 0 {
		c.addLog(LogPlayer, "Sacrifice consumes minion (%d HP)", consumeHP)
	} else {
		c.addLog(LogPlayer, "Sacrifice -%d minion HP (%d/%d)", consumeHP, nearest.HP, nearest.MaxHP)
	}
	c.DamageNearest(dmg, runes.Physical, maxRange)
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
	c.addLog(LogPlayer, "Lose %d HP (HP %d/%d)", amount, c.PlayerHP, c.PlayerMaxHP)
	if c.PlayerHP == 0 {
		c.Phase = PhaseLost
	}
}

func (c *Combat) AddEnergy(amount int) {
	c.Energy += amount
	c.addLog(LogPlayer, "+%d energy (now %d)", amount, c.Energy)
}

// PlaceWall creates a wall segment of the given length and HP, centered at
// (cx, cy) and oriented perpendicular to the line from the player to (cx, cy).
// The card click point becomes the midpoint, so a fresh Wall of Stone stands
// across incoming aggression.
func (c *Combat) PlaceWall(cx, cy float64, length float64, hp int) {
	dx := cx - c.Player.X
	dy := cy - c.Player.Y
	d := math.Hypot(dx, dy)
	if d == 0 {
		dx, dy, d = 1, 0, 1
	}
	// perpendicular unit vector
	px := -dy / d
	py := dx / d
	half := length / 2
	c.Walls = append(c.Walls, &Wall{
		X1: cx - px*half, Y1: cy - py*half,
		X2: cx + px*half, Y2: cy + py*half,
		HP: hp, MaxHP: hp,
	})
	c.markGridDirty()
	c.addLog(LogPlayer, "Wall raised (%d HP)", hp)
}

func (c *Combat) compactWalls() {
	before := len(c.Walls)
	out := c.Walls[:0]
	for _, w := range c.Walls {
		if w.HP > 0 {
			out = append(out, w)
		}
	}
	c.Walls = out
	if len(c.Walls) != before {
		c.markGridDirty()
	}
}

// --- Mesmer: rune copy ---

// buildIntentRune produces a one-shot Card representing the enemy's next
// damaging action, or false if there's nothing to copy (e.g. approach).
func (c *Combat) buildIntentRune(e *enemies.Enemy) (runes.Card, bool) {
	if e == nil || e.HP <= 0 || e.Stunned > 0 {
		return runes.Card{}, false
	}
	t := c.chooseTarget(e)
	dist := math.Hypot(e.X-t.x, e.Y-t.y)
	const mimicRange = 200.0
	if e.RangedPower > 0 {
		power := e.RangedPower
		dt := e.RangedType
		return runes.Card{
			Name:        fmt.Sprintf("Mimic: %s Cast", e.RangedType),
			Glyph:       "↻",
			Cost:        1,
			Range:       mimicRange,
			Description: fmt.Sprintf("Deal %d %s damage to the nearest enemy in range.", power, dt),
			Effect:      func(w runes.World) { w.DamageNearest(power, dt, mimicRange) },
		}, true
	}
	if dist <= e.MeleeRange {
		power := e.AttackPower
		return runes.Card{
			Name:        "Mimic: Strike",
			Glyph:       "↻",
			Cost:        1,
			Range:       mimicRange,
			Description: fmt.Sprintf("Deal %d damage to the nearest enemy in range.", power),
			Effect:      func(w runes.World) { w.DamageNearest(power, runes.Physical, mimicRange) },
		}, true
	}
	return runes.Card{}, false
}

func (c *Combat) NearestHasIntentRune(maxRange float64) bool {
	target := c.nearestNonStunnedInRange(maxRange)
	if target == nil {
		return false
	}
	_, ok := c.buildIntentRune(target)
	return ok
}

func (c *Combat) CopyNearestIntent(maxRange float64) {
	target := c.nearestNonStunnedInRange(maxRange)
	if target == nil {
		c.addLog(LogPlayer, "Mimic finds no target")
		return
	}
	card, ok := c.buildIntentRune(target)
	if !ok {
		c.addLog(LogPlayer, "Mimic: %s has no rune to copy", target.Name)
		return
	}
	c.Hand = append(c.Hand, card)
	c.addLog(LogPlayer, "Mimic copies %s into hand", card.Name)
}

func (c *Combat) nearestNonStunnedInRange(maxRange float64) *enemies.Enemy {
	var target *enemies.Enemy
	best := math.Inf(1)
	for _, e := range c.Enemies {
		if e.HP <= 0 || e.Stunned > 0 {
			continue
		}
		d := math.Hypot(e.X-c.Player.X, e.Y-c.Player.Y)
		if maxRange > 0 && d > maxRange {
			continue
		}
		if !c.hasLOS(c.Player.X, c.Player.Y, e.X, e.Y) {
			continue
		}
		if d < best {
			best = d
			target = e
		}
	}
	return target
}

func (c *Combat) nearestNonStunned() *enemies.Enemy {
	var target *enemies.Enemy
	best := math.Inf(1)
	for _, e := range c.Enemies {
		if e.HP <= 0 || e.Stunned > 0 {
			continue
		}
		d := math.Hypot(e.X-c.Player.X, e.Y-c.Player.Y)
		if d < best {
			best = d
			target = e
		}
	}
	return target
}
