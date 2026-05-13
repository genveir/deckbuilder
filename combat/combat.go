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
	EnergyAtTurn1    = 2
	EnergyCap        = 5
	HandSize         = 5
	BaseMovement     = 40
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

// Player tracks the player's position in world coordinates and a facing
// angle (radians, standard math: 0 = +X axis, increasing counter-clockwise).
// Targeting helpers filter to enemies within ConeHalfAngle of the facing.
type Player struct {
	X, Y   float64
	Facing float64
}

// ConeHalfAngle is the half-angle of the player's targeting cone in radians.
// Total cone width = 2 × ConeHalfAngle.
const ConeHalfAngle = math.Pi / 3 // 60° each side, 120° total

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
	// Dashed is set when the player has used their once-per-turn dash. The
	// dash adds BaseMovement to the budget and forfeits the spell action.
	Dashed bool

	// pendingSlowSpell holds a slow-cast spell awaiting resolution after the
	// enemy phase. Set when the player casts a spell containing any Slow rune.
	pendingSlowSpell []StagedCard

	// activeModifiers is the set of modifier rune names attached to the Core
	// currently being resolved. Set by resolveSpell around the Core's Effect.
	activeModifiers []string

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
	c.faceNearestEnemy()
	c.startPlayerTurn()
	return c
}

// faceNearestEnemy aims the player at the closest living enemy. Called once
// at combat start so every encounter opens with at least one enemy in the cone.
func (c *Combat) faceNearestEnemy() {
	var nearest *enemies.Enemy
	best := math.Inf(1)
	for _, e := range c.Enemies {
		if e.HP <= 0 {
			continue
		}
		d := math.Hypot(e.X-c.Player.X, e.Y-c.Player.Y)
		if d < best {
			best = d
			nearest = e
		}
	}
	if nearest != nil {
		c.Player.Facing = math.Atan2(nearest.Y-c.Player.Y, nearest.X-c.Player.X)
	}
}

// inCone reports whether (tx, ty) lies within the player's targeting cone.
func (c *Combat) inCone(tx, ty float64) bool {
	dx := tx - c.Player.X
	dy := ty - c.Player.Y
	if dx == 0 && dy == 0 {
		return true
	}
	angle := math.Atan2(dy, dx)
	delta := angle - c.Player.Facing
	// wrap to [-pi, pi]
	for delta > math.Pi {
		delta -= 2 * math.Pi
	}
	for delta < -math.Pi {
		delta += 2 * math.Pi
	}
	if delta < 0 {
		delta = -delta
	}
	return delta <= ConeHalfAngle
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
	max := EnergyAtTurn1 + c.Turn - 1
	if max > EnergyCap {
		max = EnergyCap
	}
	c.MaxEnergy = max
	c.Energy = max
	// PlayerArmor persists across turns; only consumed by incoming damage.
	c.MovementBudget = BaseMovement
	c.hasMoved = false
	c.Stage = c.Stage[:0]
	c.SpellCast = false
	c.Dashed = false
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

// StageCard adds a rune to the spell being composed this turn. Energy is NOT
// deducted at stage time — only at cast time. Staging only verifies the spell
// CAN be afforded (sum of staged costs + this card <= Energy).
func (c *Combat) StageCard(i int) (staged, needsPlacement bool) {
	if c.Phase != PhasePlayer || c.SpellCast || c.PendingCardIdx >= 0 {
		return false, false
	}
	if i < 0 || i >= len(c.Hand) {
		return false, false
	}
	card := c.Hand[i]
	if c.TotalStagedCost()+card.Cost > c.Energy {
		return false, false
	}
	if ok, _ := c.CanAddToStage(card); !ok {
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
	c.Stage = append(c.Stage, StagedCard{Card: card})
	c.Hand = append(c.Hand[:i], c.Hand[i+1:]...)
	return true, false
}

// TotalStagedCost is the sum of base costs of every staged rune. Does NOT
// include modifier-driven resolve-time costs (e.g. Disperse), which are paid
// when the spell actually casts.
func (c *Combat) TotalStagedCost() int {
	sum := 0
	for _, sc := range c.Stage {
		sum += sc.Card.Cost
	}
	return sum
}

// CanAddToStage enforces the spell-composition grammar:
//   - at most one Core per spell;
//   - a Modifier requires a staged Core of matching Family;
//   - the same Modifier (by name) can only be staged once per spell.
//
// Returns (true, "") if the card may be added, otherwise (false, reason).
func (c *Combat) CanAddToStage(card runes.Card) (bool, string) {
	if card.Role == runes.RoleModifier {
		hasMatch := false
		for _, sc := range c.Stage {
			if sc.Card.Role == runes.RoleCore && sc.Card.Family == card.ModifiesFamily {
				hasMatch = true
				break
			}
		}
		if !hasMatch {
			return false, "no compatible core rune in spell"
		}
		for _, sc := range c.Stage {
			if sc.Card.Role == runes.RoleModifier && sc.Card.Name == card.Name {
				return false, "modifier already attached"
			}
		}
		return true, ""
	}
	// Core
	for _, sc := range c.Stage {
		if sc.Card.Role == runes.RoleCore {
			return false, "spell already has a core rune"
		}
	}
	return true, ""
}

// ConfirmPlacement stages the pending card with the chosen world target. As
// with StageCard, no energy is deducted — that happens at cast time.
func (c *Combat) ConfirmPlacement(x, y float64) bool {
	if c.PendingCardIdx < 0 || c.PendingCardIdx >= len(c.Hand) {
		c.PendingCardIdx = -1
		return false
	}
	i := c.PendingCardIdx
	card := c.Hand[i]
	c.Stage = append(c.Stage, StagedCard{Card: card, PlaceX: x, PlaceY: y})
	c.Hand = append(c.Hand[:i], c.Hand[i+1:]...)
	c.PendingCardIdx = -1
	return true
}

func (c *Combat) CancelPlacement() { c.PendingCardIdx = -1 }

// CanDash reports whether the player may dash this turn — no staged runes,
// no spell cast yet, no prior dash.
func (c *Combat) CanDash() bool {
	return c.Phase == PhasePlayer && !c.Dashed && !c.SpellCast &&
		len(c.Stage) == 0 && c.PendingCardIdx < 0
}

// Dash grants +BaseMovement movement budget for the turn and forfeits the
// spell action (no staging or casting afterwards). Returns false if not
// available.
func (c *Combat) Dash() bool {
	if !c.CanDash() {
		return false
	}
	c.MovementBudget += BaseMovement
	c.Dashed = true
	c.SpellCast = true
	c.addLog(LogPlayer, "Dash (+%d movement, no spell)", int(BaseMovement))
	return true
}

// UnstageLast returns the most recently staged rune to the player's hand.
// No energy refund needed — costs are only paid at cast time.
func (c *Combat) UnstageLast() bool {
	if c.SpellCast || len(c.Stage) == 0 {
		return false
	}
	last := c.Stage[len(c.Stage)-1]
	c.Stage = c.Stage[:len(c.Stage)-1]
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

// CastSpell pays the spell's base cost (sum of staged runes), then resolves.
// If any rune is Slow, the spell is queued to resolve after the upcoming enemy
// phase; the player turn ends immediately. Modifier-driven runtime costs (e.g.
// Disperse) are paid inside the Core's Effect via SpendEnergy.
func (c *Combat) CastSpell() bool {
	if c.Phase != PhasePlayer || c.SpellCast || len(c.Stage) == 0 {
		return false
	}
	cost := c.TotalStagedCost()
	if c.Energy < cost {
		// Stage-time validation should make this unreachable; defensive only.
		return false
	}
	c.Energy -= cost

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

// resolveSpell processes a composed spell: extracts the Core and Modifier
// names, sets activeModifiers for the duration of the Core's effect, and
// discards every rune to the player's discard pile.
func (c *Combat) resolveSpell(stage []StagedCard) {
	var core *StagedCard
	mods := mods0[:0]
	for i := range stage {
		sc := &stage[i]
		if sc.Card.Role == runes.RoleModifier {
			mods = append(mods, sc.Card.Name)
			continue
		}
		if core == nil {
			core = sc
		}
	}
	if core != nil {
		c.activeModifiers = mods
		switch {
		case core.Card.Effect != nil:
			core.Card.Effect(c)
		case core.Card.PlacementEffect != nil:
			core.Card.PlacementEffect(c, core.PlaceX, core.PlaceY)
		}
		c.activeModifiers = nil
	}
	for _, sc := range stage {
		c.Discard = append(c.Discard, sc.Card)
	}
}

// reusable scratch slice for modifier names during resolution
var mods0 = make([]string, 0, 4)

// MoveTowards moves the player along the given offset (radar-relative delta),
// consuming movement budget. Walls block movement: the player stops just
// before a wall they would otherwise cross. Facing always rotates to the
// chosen direction, even when movement is blocked or budget is 0. Returns the
// distance actually moved.
func (c *Combat) MoveTowards(dx, dy float64) float64 {
	if c.Phase != PhasePlayer {
		return 0
	}
	dist := math.Hypot(dx, dy)
	if dist == 0 {
		return 0
	}
	c.Player.Facing = math.Atan2(dy, dx)
	if c.MovementBudget <= 0 {
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

	// Turn options, in priority order:
	//   1) act from current position (attack / cast)
	//   2) move 1x and act (move-attack / move-cast)
	//   3) dash 2x (no attack)
	//   4) wall-smash fallback
	dashStep := e.MoveSpeed * 2

	if c.enemyCanActFrom(e, e.X, e.Y, t) {
		if e.RangedPower > 0 {
			apply(e.RangedPower, e.RangedType)
			e.Intent = "cast"
		} else {
			apply(e.AttackPower, runes.Physical)
			e.Intent = "attacked"
		}
		return
	}

	if ok, nx, ny := c.enemyCanMoveThenAct(e, t); ok {
		e.X, e.Y = nx, ny
		if e.RangedPower > 0 {
			apply(e.RangedPower, e.RangedType)
			e.Intent = "move + cast"
		} else {
			apply(e.AttackPower, runes.Physical)
			e.Intent = "move + attack"
		}
		return
	}

	// Out of attack reach this turn — dash.
	if path := c.findPath(e.X, e.Y, t.x, t.y); len(path) >= 2 {
		nx, ny := c.smoothPathStep(e.X, e.Y, path, dashStep)
		moveTowardPoint(nx, ny, dashStep)
		e.Intent = "dash"
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
		step := math.Min(distToWall-e.MeleeRange, dashStep)
		if step > 0 {
			moveTowardPoint(cpx, cpy, step)
			e.Intent = "dash to wall"
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

// enemyCanActFrom reports whether the enemy could attack/cast at target t from
// the hypothetical position (px, py).
func (c *Combat) enemyCanActFrom(e *enemies.Enemy, px, py float64, t aggroTarget) bool {
	d := math.Hypot(px-t.x, py-t.y)
	if e.RangedPower > 0 {
		if e.MaxRange > 0 && d > e.MaxRange {
			return false
		}
		return c.hasLOS(px, py, t.x, t.y)
	}
	return d <= e.MeleeRange && c.hasLOS(px, py, t.x, t.y)
}

// enemyCanMoveThenAct simulates a single normal-speed move along the A* path
// and reports whether the enemy could act from the resulting position.
func (c *Combat) enemyCanMoveThenAct(e *enemies.Enemy, t aggroTarget) (bool, float64, float64) {
	path := c.findPath(e.X, e.Y, t.x, t.y)
	if len(path) < 2 {
		return false, e.X, e.Y
	}
	nx, ny := c.smoothPathStep(e.X, e.Y, path, e.MoveSpeed)
	ddx := nx - e.X
	ddy := ny - e.Y
	n := math.Hypot(ddx, ddy)
	if n == 0 {
		return false, e.X, e.Y
	}
	s := math.Min(e.MoveSpeed, n)
	px := e.X + ddx/n*s
	py := e.Y + ddy/n*s
	if c.enemyCanActFrom(e, px, py, t) {
		return true, px, py
	}
	return false, e.X, e.Y
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
		_ = dist
		if c.enemyCanActFrom(e, e.X, e.Y, t) {
			if e.RangedPower > 0 {
				e.Intent = fmt.Sprintf("cast %s%s (%d)", e.RangedType, suffix, e.RangedPower)
			} else {
				e.Intent = fmt.Sprintf("attack%s (%d)", suffix, e.AttackPower)
			}
			continue
		}
		if ok, _, _ := c.enemyCanMoveThenAct(e, t); ok {
			if e.RangedPower > 0 {
				e.Intent = fmt.Sprintf("move + cast %s%s (%d)", e.RangedType, suffix, e.RangedPower)
			} else {
				e.Intent = fmt.Sprintf("move + attack%s (%d)", suffix, e.AttackPower)
			}
			continue
		}
		if path := c.findPath(e.X, e.Y, t.x, t.y); len(path) >= 2 {
			e.Intent = "dash"
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
// maxRange of the player, in the player's targeting cone, and has
// line-of-sight. maxRange of 0 means unlimited.
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
		if !c.inCone(e.X, e.Y) {
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

// CountTargetsInRange returns how many living enemies are within maxRange of
// the player with line-of-sight. Cone is ignored — matches DamageAll's filter.
// Used by Disperse-modified spells to compute per-target energy cost.
func (c *Combat) CountTargetsInRange(maxRange float64) int {
	n := 0
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
		n++
	}
	return n
}

// SpendEnergy deducts amount energy if available, returning false if the
// player doesn't have enough (in which case no energy is spent).
func (c *Combat) SpendEnergy(amount int) bool {
	if amount < 0 {
		return false
	}
	if c.Energy < amount {
		return false
	}
	c.Energy -= amount
	return true
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

// NearestIntendsAttack returns true if the cone-nearest living enemy within
// maxRange (0 = unlimited) will deal damage to the player on its next turn —
// either by acting from its current position OR by moving and attacking
// (matching the enemy decision tree in runEnemyProgram).
func (c *Combat) NearestIntendsAttack(maxRange float64) bool {
	target := c.nearestEnemyInRange(maxRange)
	if target == nil {
		return false
	}
	if target.Stunned > 0 {
		return false
	}
	t := c.chooseTarget(target)
	if t.isMinion() {
		return false
	}
	if c.enemyCanActFrom(target, target.X, target.Y, t) {
		return true
	}
	if ok, _, _ := c.enemyCanMoveThenAct(target, t); ok {
		return true
	}
	return false
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

// HasModifier reports whether a modifier of the given name is attached to the
// Core currently being resolved. Only meaningful from within a Core's Effect.
func (c *Combat) HasModifier(name string) bool {
	for _, m := range c.activeModifiers {
		if m == name {
			return true
		}
	}
	return false
}

// LogNote lets a rune Effect add a free-form line to the combat log.
func (c *Combat) LogNote(text string) {
	c.addLog(LogPlayer, "%s", text)
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
		if !c.inCone(e.X, e.Y) {
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
